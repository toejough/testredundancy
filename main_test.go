package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestParseBlockID(t *testing.T) {
	tests := []struct {
		name      string
		blockID   string
		wantFile  string
		wantStart [2]int // line, col
		wantEnd   [2]int // line, col
		wantErr   bool
	}{
		{
			name:      "valid block",
			blockID:   "github.com/foo/bar/file.go:10.5,20.10",
			wantFile:  "github.com/foo/bar/file.go",
			wantStart: [2]int{10, 5},
			wantEnd:   [2]int{20, 10},
		},
		{
			name:      "simple file",
			blockID:   "main.go:1.1,5.2",
			wantFile:  "main.go",
			wantStart: [2]int{1, 1},
			wantEnd:   [2]int{5, 2},
		},
		{
			name:    "missing colon",
			blockID: "file.go10.5,20.10",
			wantErr: true,
		},
		{
			name:    "missing comma",
			blockID: "file.go:10.5-20.10",
			wantErr: true,
		},
		{
			name:    "invalid start format",
			blockID: "file.go:10,20.10",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, startLine, startCol, endLine, endCol, err := parseBlockID(tt.blockID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if startLine != tt.wantStart[0] {
				t.Errorf("startLine = %d, want %d", startLine, tt.wantStart[0])
			}
			if startCol != tt.wantStart[1] {
				t.Errorf("startCol = %d, want %d", startCol, tt.wantStart[1])
			}
			if endLine != tt.wantEnd[0] {
				t.Errorf("endLine = %d, want %d", endLine, tt.wantEnd[0])
			}
			if endCol != tt.wantEnd[1] {
				t.Errorf("endCol = %d, want %d", endCol, tt.wantEnd[1])
			}
		})
	}
}

func TestParseCoverageBlock(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantBlock  coverageBlock
		wantErr    bool
	}{
		{
			name: "valid line",
			line: "github.com/foo/bar.go:10.5,20.10 3 1",
			wantBlock: coverageBlock{
				file:       "github.com/foo/bar.go",
				startLine:  10,
				startCol:   5,
				endLine:    20,
				endCol:     10,
				statements: 3,
				count:      1,
			},
		},
		{
			name: "zero count",
			line: "main.go:1.1,5.2 2 0",
			wantBlock: coverageBlock{
				file:       "main.go",
				startLine:  1,
				startCol:   1,
				endLine:    5,
				endCol:     2,
				statements: 2,
				count:      0,
			},
		},
		{
			name:    "missing fields",
			line:    "main.go:1.1,5.2 2",
			wantErr: true,
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := parseCoverageBlock(tt.line)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if block != tt.wantBlock {
				t.Errorf("block = %+v, want %+v", block, tt.wantBlock)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
		{"with:colon", "with_colon"},
		{"with.dot", "with_dot"},
		{"github.com/foo/bar", "github_com_foo_bar"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitize(tt.input)
			if got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasParallelCall(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{
			name: "has t.Parallel",
			code: `func TestFoo(t *testing.T) {
				t.Parallel()
				// test code
			}`,
			want: true,
		},
		{
			name: "no parallel",
			code: `func TestFoo(t *testing.T) {
				// test code
			}`,
			want: false,
		},
		{
			name: "parallel in subtest",
			code: `func TestFoo(t *testing.T) {
				t.Run("sub", func(t *testing.T) {
					t.Parallel()
				})
			}`,
			want: true,
		},
		{
			name: "other method call",
			code: `func TestFoo(t *testing.T) {
				t.Helper()
				t.Log("hello")
			}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the function in a package for parsing
			src := "package test\n\nimport \"testing\"\n\n" + tt.code

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", src, 0)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			// Find the function body
			var body *ast.BlockStmt
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					body = fn.Body
					break
				}
			}

			if body == nil {
				t.Fatal("no function body found")
			}

			got := hasParallelCall(body)
			if got != tt.want {
				t.Errorf("hasParallelCall() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeCoverageBlocksFile(t *testing.T) {
	// Create a temp file with duplicate blocks
	content := `mode: set
github.com/foo/bar.go:10.5,20.10 3 1
github.com/foo/bar.go:10.5,20.10 3 1
github.com/foo/bar.go:25.1,30.5 2 0
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "coverage.out")

	if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Merge the blocks
	if err := mergeCoverageBlocksFile(tmpFile); err != nil {
		t.Fatalf("mergeCoverageBlocksFile() error: %v", err)
	}

	// Read the result
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	result := string(data)

	// Should have mode line
	if !contains(result, "mode: set") {
		t.Error("missing mode line")
	}

	// The duplicate block should be merged (count summed: 1+1=2)
	if !contains(result, "github.com/foo/bar.go:10.5,20.10 3 2") {
		t.Error("duplicate blocks not merged correctly, expected count=2")
	}

	// The non-duplicate block should remain unchanged
	if !contains(result, "github.com/foo/bar.go:25.1,30.5 2 0") {
		t.Error("non-duplicate block should remain")
	}
}

func TestMergeMultipleCoverageFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first coverage file
	file1 := filepath.Join(tmpDir, "cov1.out")
	content1 := `mode: set
github.com/foo/bar.go:10.5,20.10 3 1
github.com/foo/bar.go:25.1,30.5 2 0
`
	if err := os.WriteFile(file1, []byte(content1), 0600); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	// Create second coverage file
	file2 := filepath.Join(tmpDir, "cov2.out")
	content2 := `mode: set
github.com/foo/bar.go:10.5,20.10 3 1
github.com/foo/bar.go:35.1,40.5 4 1
`
	if err := os.WriteFile(file2, []byte(content2), 0600); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	// Merge the files
	outputFile := filepath.Join(tmpDir, "merged.out")
	if err := mergeMultipleCoverageFiles([]string{file1, file2}, outputFile); err != nil {
		t.Fatalf("mergeMultipleCoverageFiles() error: %v", err)
	}

	// Read the result
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	result := string(data)

	// Should have mode line
	if !contains(result, "mode: set") {
		t.Error("missing mode line")
	}

	// Block from both files should be merged (count summed: 1+1=2)
	if !contains(result, "github.com/foo/bar.go:10.5,20.10 3 2") {
		t.Error("overlapping blocks not merged correctly, expected count=2")
	}

	// Block unique to file1 should remain
	if !contains(result, "github.com/foo/bar.go:25.1,30.5 2 0") {
		t.Error("block from file1 missing")
	}

	// Block unique to file2 should remain
	if !contains(result, "github.com/foo/bar.go:35.1,40.5 4 1") {
		t.Error("block from file2 missing")
	}
}

func TestFilterQtplFromCoverage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create coverage file with .qtpl entries
	inputFile := filepath.Join(tmpDir, "cov.out")
	content := `mode: set
github.com/foo/bar.go:10.5,20.10 3 1
github.com/foo/template.qtpl:5.1,10.5 2 1
github.com/foo/other.go:1.1,5.5 1 0
`
	if err := os.WriteFile(inputFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	// Filter out .qtpl
	outputFile := filepath.Join(tmpDir, "filtered.out")
	if err := filterQtplFromCoverage(inputFile, outputFile); err != nil {
		t.Fatalf("filterQtplFromCoverage() error: %v", err)
	}

	// Read the result
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	result := string(data)

	// Should have mode line
	if !contains(result, "mode: set") {
		t.Error("missing mode line")
	}

	// Should keep non-.qtpl entries
	if !contains(result, "github.com/foo/bar.go") {
		t.Error("non-.qtpl entry should remain")
	}
	if !contains(result, "github.com/foo/other.go") {
		t.Error("non-.qtpl entry should remain")
	}

	// Should NOT have .qtpl entry
	if contains(result, ".qtpl") {
		t.Error(".qtpl entries should be filtered out")
	}
}

func TestTestInfoQualifiedName(t *testing.T) {
	info := testInfo{pkg: "github.com/foo/bar", name: "TestSomething"}
	got := info.qualifiedName()
	want := "github.com/foo/bar:TestSomething"
	if got != want {
		t.Errorf("qualifiedName() = %q, want %q", got, want)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
