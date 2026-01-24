package coverage

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FunctionBounds represents the line range of a function in a source file.
type FunctionBounds struct {
	Name      string // Function name (e.g., "Foo" or "(*T).Method")
	StartLine int
	EndLine   int
}

// FunctionMap maps file paths to their function boundaries.
// Key is the full path as it appears in coverage files (e.g., "github.com/foo/bar/file.go")
type FunctionMap map[string][]FunctionBounds

// BuildFunctionMap parses Go source files to extract function boundaries.
// It takes a module path (e.g., "github.com/foo/bar") and source directory.
func BuildFunctionMap(moduleRoot string) (FunctionMap, error) {
	funcMap := make(FunctionMap)

	// Read go.mod to get module path
	goModPath := filepath.Join(moduleRoot, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod at %s: %w", goModPath, err)
	}

	modulePath := extractModulePath(string(goModContent))
	if modulePath == "" {
		return nil, fmt.Errorf("could not extract module path from go.mod at %s", goModPath)
	}

	// Walk the source tree
	err = filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor, testdata, and hidden directories (but not "." itself)
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "testdata" || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files (skip tests)
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse the file
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			// Skip files that don't parse (might be build-constrained)
			return nil
		}

		// Build the coverage file path (module path + relative path)
		relPath, _ := filepath.Rel(moduleRoot, path)
		coverPath := modulePath + "/" + filepath.ToSlash(relPath)

		// Extract function bounds
		var bounds []FunctionBounds
		ast.Inspect(file, func(n ast.Node) bool {
			switch fn := n.(type) {
			case *ast.FuncDecl:
				name := fn.Name.Name
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					// Method - include receiver type
					recvType := exprToString(fn.Recv.List[0].Type)
					name = "(" + recvType + ")." + name
				}

				startLine := fset.Position(fn.Pos()).Line
				endLine := fset.Position(fn.End()).Line

				bounds = append(bounds, FunctionBounds{
					Name:      name,
					StartLine: startLine,
					EndLine:   endLine,
				})
			}
			return true
		})

		if len(bounds) > 0 {
			// Sort by start line for efficient lookup
			sort.Slice(bounds, func(i, j int) bool {
				return bounds[i].StartLine < bounds[j].StartLine
			})
			funcMap[coverPath] = bounds
		}

		return nil
	})

	return funcMap, err
}

// extractModulePath extracts the module path from go.mod content.
func extractModulePath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// exprToString converts a receiver type expression to a string.
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.IndexExpr:
		return exprToString(t.X) + "[" + exprToString(t.Index) + "]"
	case *ast.IndexListExpr:
		// Generic type with multiple type params
		return exprToString(t.X)
	default:
		return "?"
	}
}

// FindFunction returns the function name containing the given line in a file.
// Returns empty string if no function contains the line.
func (fm FunctionMap) FindFunction(file string, line int) string {
	bounds, ok := fm[file]
	if !ok {
		return ""
	}

	// Binary search for the function containing this line
	idx := sort.Search(len(bounds), func(i int) bool {
		return bounds[i].EndLine >= line
	})

	if idx < len(bounds) && bounds[idx].StartLine <= line && line <= bounds[idx].EndLine {
		return file + ":" + bounds[idx].Name
	}

	return ""
}

// ComputeFunctionCoverage computes per-function coverage from a BlockSet.
// Returns a map of function name -> coverage percentage.
func (fm FunctionMap) ComputeFunctionCoverage(bs *BlockSet) map[string]float64 {
	// Track statements per function
	type funcStats struct {
		covered int
		total   int
	}
	stats := make(map[string]*funcStats)

	for blockID, info := range bs.Blocks {
		// Parse block ID: "file.go:startLine.startCol,endLine.endCol"
		file, startLine, _, _, _, err := ParseBlockID(blockID)
		if err != nil {
			continue
		}

		// Find the function containing this block
		funcName := fm.FindFunction(file, startLine)
		if funcName == "" {
			continue
		}

		// Update stats
		if stats[funcName] == nil {
			stats[funcName] = &funcStats{}
		}
		stats[funcName].total += info.Statements
		if info.Covered {
			stats[funcName].covered += info.Statements
		}
	}

	// Convert to percentages
	result := make(map[string]float64)
	for fn, s := range stats {
		if s.total > 0 {
			result[fn] = float64(s.covered) * 100.0 / float64(s.total)
		}
	}

	return result
}
