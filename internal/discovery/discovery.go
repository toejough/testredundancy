// Package discovery provides utilities for discovering and inspecting Go tests.
package discovery

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	executil "github.com/toejough/testredundancy/internal/exec"
)

// TestInfo contains information about a discovered test.
type TestInfo struct {
	Pkg  string
	Name string
}

// QualifiedName returns the package-qualified test name (pkg:TestName).
func (t TestInfo) QualifiedName() string {
	return t.Pkg + ":" + t.Name
}

// ListTests lists all test functions with their packages for the given package pattern.
func ListTests(pkgPattern string) ([]TestInfo, error) {
	// First, expand the package pattern to get actual packages
	listOut, err := executil.Output(context.Background(), "go", "list", pkgPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	var allTests []TestInfo
	packages := strings.Split(strings.TrimSpace(listOut), "\n")

	for _, pkg := range packages {
		if pkg == "" {
			continue
		}

		out, err := executil.Output(context.Background(), "go", "test", "-list", ".", pkg)
		if err != nil {
			// Package may have no tests, skip it
			continue
		}

		lines := strings.Split(out, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Test") {
				allTests = append(allTests, TestInfo{Pkg: pkg, Name: line})
			}
		}
	}

	return allTests, nil
}

// DetectParallelTests detects which tests are marked with t.Parallel().
// Returns a map of qualified test names (pkg:TestName) that are parallel-safe.
func DetectParallelTests(tests []TestInfo) map[string]bool {
	result := make(map[string]bool)

	// Group tests by package
	testsByPkg := make(map[string][]TestInfo)
	for _, t := range tests {
		testsByPkg[t.Pkg] = append(testsByPkg[t.Pkg], t)
	}

	for pkg, pkgTests := range testsByPkg {
		// Get the directory for this package
		pkgDir, err := executil.Output(context.Background(), "go", "list", "-f", "{{.Dir}}", pkg)
		if err != nil {
			continue
		}

		pkgDir = strings.TrimSpace(pkgDir)

		// Find test files in this package
		testFiles, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
		if err != nil {
			continue
		}

		// Parse each test file and check for t.Parallel() calls
		fset := token.NewFileSet()

		for _, testFile := range testFiles {
			f, err := parser.ParseFile(fset, testFile, nil, parser.ParseComments)
			if err != nil {
				continue
			}

			// Find all test functions and check if they call t.Parallel()
			ast.Inspect(f, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Name == nil {
					return true
				}

				// Only consider Test* functions with *testing.T parameter
				if !strings.HasPrefix(fn.Name.Name, "Test") {
					return true
				}

				// Check if this function is one we care about
				qualifiedName := pkg + ":" + fn.Name.Name
				isRelevant := false

				for _, t := range pkgTests {
					if t.QualifiedName() == qualifiedName {
						isRelevant = true

						break
					}
				}

				if !isRelevant {
					return true
				}

				// Check if function body contains t.Parallel() call
				if fn.Body != nil && HasParallelCall(fn.Body) {
					result[qualifiedName] = true
				}

				return true
			})
		}
	}

	return result
}

// HasParallelCall checks if a block statement contains a call to t.Parallel().
func HasParallelCall(body *ast.BlockStmt) bool {
	found := false

	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for selector expression (t.Parallel or something.Parallel)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check if the method is "Parallel"
		if sel.Sel.Name == "Parallel" {
			found = true

			return false
		}

		return true
	})

	return found
}
