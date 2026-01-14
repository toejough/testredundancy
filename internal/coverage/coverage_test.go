package coverage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toejough/testredundancy/internal/coverage"
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
			file, startLine, startCol, endLine, endCol, err := coverage.ParseBlockID(tt.blockID)

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

func TestParseBlock(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantBlock coverage.Block
		wantErr   bool
	}{
		{
			name: "valid line",
			line: "github.com/foo/bar.go:10.5,20.10 3 1",
			wantBlock: coverage.Block{
				File:       "github.com/foo/bar.go",
				StartLine:  10,
				StartCol:   5,
				EndLine:    20,
				EndCol:     10,
				Statements: 3,
				Count:      1,
			},
		},
		{
			name: "zero count",
			line: "main.go:1.1,5.2 2 0",
			wantBlock: coverage.Block{
				File:       "main.go",
				StartLine:  1,
				StartCol:   1,
				EndLine:    5,
				EndCol:     2,
				Statements: 2,
				Count:      0,
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
			block, err := coverage.ParseBlock(tt.line)

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

func TestMergeBlocksFile(t *testing.T) {
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
	if err := coverage.MergeBlocksFile(tmpFile); err != nil {
		t.Fatalf("MergeBlocksFile() error: %v", err)
	}

	// Read the result
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	result := string(data)

	// Should have mode line
	if !strings.Contains(result, "mode: set") {
		t.Error("missing mode line")
	}

	// The duplicate block should be merged (count summed: 1+1=2)
	if !strings.Contains(result, "github.com/foo/bar.go:10.5,20.10 3 2") {
		t.Error("duplicate blocks not merged correctly, expected count=2")
	}

	// The non-duplicate block should remain unchanged
	if !strings.Contains(result, "github.com/foo/bar.go:25.1,30.5 2 0") {
		t.Error("non-duplicate block should remain")
	}
}

func TestMergeFiles(t *testing.T) {
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
	if err := coverage.MergeFiles([]string{file1, file2}, outputFile); err != nil {
		t.Fatalf("MergeFiles() error: %v", err)
	}

	// Read the result
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	result := string(data)

	// Should have mode line
	if !strings.Contains(result, "mode: set") {
		t.Error("missing mode line")
	}

	// Block from both files should be merged (count summed: 1+1=2)
	if !strings.Contains(result, "github.com/foo/bar.go:10.5,20.10 3 2") {
		t.Error("overlapping blocks not merged correctly, expected count=2")
	}

	// Block unique to file1 should remain
	if !strings.Contains(result, "github.com/foo/bar.go:25.1,30.5 2 0") {
		t.Error("block from file1 missing")
	}

	// Block unique to file2 should remain
	if !strings.Contains(result, "github.com/foo/bar.go:35.1,40.5 4 1") {
		t.Error("block from file2 missing")
	}
}

func TestFilterQtpl(t *testing.T) {
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
	if err := coverage.FilterQtpl(inputFile, outputFile); err != nil {
		t.Fatalf("FilterQtpl() error: %v", err)
	}

	// Read the result
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	result := string(data)

	// Should have mode line
	if !strings.Contains(result, "mode: set") {
		t.Error("missing mode line")
	}

	// Should keep non-.qtpl entries
	if !strings.Contains(result, "github.com/foo/bar.go") {
		t.Error("non-.qtpl entry should remain")
	}
	if !strings.Contains(result, "github.com/foo/other.go") {
		t.Error("non-.qtpl entry should remain")
	}

	// Should NOT have .qtpl entry
	if strings.Contains(result, ".qtpl") {
		t.Error(".qtpl entries should be filtered out")
	}
}
