// Package coverage provides utilities for working with Go coverage files.
package coverage

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Block represents a coverage block in a coverage file.
type Block struct {
	File       string
	StartLine  int
	StartCol   int
	EndLine    int
	EndCol     int
	Statements int
	Count      int
}

// FilterQtpl removes .qtpl template file entries from a coverage file.
func FilterQtpl(inputFile, outputFile string) error {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", inputFile, err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("empty coverage file: %s", inputFile)
	}

	// Keep mode line, filter out .qtpl entries
	filtered := []string{lines[0]} // mode line

	for _, line := range lines[1:] {
		if line == "" || strings.Contains(line, ".qtpl:") {
			continue
		}

		filtered = append(filtered, line)
	}

	result := strings.Join(filtered, "\n")

	err = os.WriteFile(outputFile, []byte(result), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", outputFile, err)
	}

	return nil
}

// GetAllFunctionsCoverage returns a map of function name -> coverage percentage for all functions.
func GetAllFunctionsCoverage(coverageFile string) (map[string]float64, error) {
	out, err := exec.Command("go", "tool", "cover", "-func="+coverageFile).Output()
	if err != nil {
		return nil, fmt.Errorf("go tool cover failed: %w", err)
	}

	funcs := make(map[string]float64)
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "total:") {
			continue
		}

		// Format: file:line:  functionName  percentage%
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Last field is percentage like "85.7%"
		percentStr := fields[len(fields)-1]
		percentStr = strings.TrimSuffix(percentStr, "%")

		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			continue
		}

		// Function name with location (e.g., "file.go:123: funcName")
		funcName := strings.Join(fields[0:len(fields)-1], " ")
		funcs[funcName] = percent
	}

	return funcs, nil
}

// MergeBlocksFile merges duplicate coverage blocks in a coverage file (in-place).
func MergeBlocksFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil
	}

	// Keep the mode line
	mode := lines[0]

	// Parse all blocks
	var blocks []Block
	blockCounts := make(map[string]int)

	for _, line := range lines[1:] {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue
		}

		blockID := parts[0]
		numStmts, _ := strconv.Atoi(parts[1])
		count, _ := strconv.Atoi(parts[2])

		file, startLine, startCol, endLine, endCol, err := ParseBlockID(blockID)
		if err != nil {
			continue
		}

		// Sum counts for identical blocks
		blockCounts[blockID] += count

		// Store block for deduplication
		found := false

		for i, b := range blocks {
			if b.File == file && b.StartLine == startLine && b.StartCol == startCol &&
				b.EndLine == endLine && b.EndCol == endCol {
				blocks[i].Count = blockCounts[blockID]
				found = true

				break
			}
		}

		if !found {
			blocks = append(blocks, Block{
				File:       file,
				StartLine:  startLine,
				StartCol:   startCol,
				EndLine:    endLine,
				EndCol:     endCol,
				Statements: numStmts,
				Count:      blockCounts[blockID],
			})
		}
	}

	// Rebuild coverage file with deduplicated blocks
	// Note: We don't split overlapping blocks - go tool cover handles them correctly.
	// We only deduplicate identical blocks (same start/end positions) by summing counts.
	var merged []string
	merged = append(merged, mode)

	// Sort for deterministic output
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].File != blocks[j].File {
			return blocks[i].File < blocks[j].File
		}

		if blocks[i].StartLine != blocks[j].StartLine {
			return blocks[i].StartLine < blocks[j].StartLine
		}

		return blocks[i].StartCol < blocks[j].StartCol
	})

	for _, block := range blocks {
		blockID := fmt.Sprintf("%s:%d.%d,%d.%d",
			block.File, block.StartLine, block.StartCol, block.EndLine, block.EndCol)
		merged = append(merged, fmt.Sprintf("%s %d %d", blockID, block.Statements, block.Count))
	}

	// Write merged coverage
	return os.WriteFile(filename, []byte(strings.Join(merged, "\n")+"\n"), 0o600)
}

// MergeFiles merges multiple coverage files into a single output file.
func MergeFiles(files []string, outputFile string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to merge")
	}

	var mode string
	var allBlocks []string

	for i, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			continue
		}

		// Use mode from first file
		if i == 0 {
			mode = lines[0]
		}

		// Append blocks from this file (skip mode line and .qtpl files)
		for _, line := range lines[1:] {
			// Skip empty lines and lines referencing .qtpl template files
			if line == "" || strings.Contains(line, ".qtpl:") {
				continue
			}

			allBlocks = append(allBlocks, line)
		}
	}

	// Write combined file
	combined := mode + "\n" + strings.Join(allBlocks, "\n")

	err := os.WriteFile(outputFile, []byte(combined), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", outputFile, err)
	}

	// Merge overlapping blocks using existing logic
	return MergeBlocksFile(outputFile)
}

// ParseBlockID parses a coverage block ID like "file.go:10.5,20.10".
func ParseBlockID(blockID string) (file string, startLine, startCol, endLine, endCol int, err error) {
	fileParts := strings.Split(blockID, ":")
	if len(fileParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid block ID format: %s", blockID)
	}

	file = fileParts[0]

	rangeParts := strings.Split(fileParts[1], ",")
	if len(rangeParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid range format: %s", blockID)
	}

	startParts := strings.Split(rangeParts[0], ".")
	if len(startParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid start position: %s", blockID)
	}

	endParts := strings.Split(rangeParts[1], ".")
	if len(endParts) != 2 {
		return "", 0, 0, 0, 0, fmt.Errorf("invalid end position: %s", blockID)
	}

	startLine, _ = strconv.Atoi(startParts[0])
	startCol, _ = strconv.Atoi(startParts[1])
	endLine, _ = strconv.Atoi(endParts[0])
	endCol, _ = strconv.Atoi(endParts[1])

	return file, startLine, startCol, endLine, endCol, nil
}

// ParseBlock parses a coverage block line like "file.go:10.5,20.10 3 1".
func ParseBlock(line string) (Block, error) {
	// Format: file:startLine.startCol,endLine.endCol statements count
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return Block{}, fmt.Errorf("invalid line format")
	}

	blockID := parts[0]
	statements, _ := strconv.Atoi(parts[1])
	count, _ := strconv.Atoi(parts[2])

	file, startLine, startCol, endLine, endCol, err := ParseBlockID(blockID)
	if err != nil {
		return Block{}, err
	}

	return Block{
		File:       file,
		StartLine:  startLine,
		StartCol:   startCol,
		EndLine:    endLine,
		EndCol:     endCol,
		Statements: statements,
		Count:      count,
	}, nil
}

// BlockSet represents coverage data as a set of covered blocks with statement counts.
// Key is block ID (e.g., "file.go:10.5,20.10"), value is (statements, covered).
type BlockSet struct {
	// Blocks maps block ID to (statements, isCovered)
	Blocks map[string]BlockInfo
}

// BlockInfo holds statement count and coverage status for a block.
type BlockInfo struct {
	Statements int
	Covered    bool
}

// ParseFileToBlockSet parses a coverage file into an in-memory BlockSet.
func ParseFileToBlockSet(filename string) (*BlockSet, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	bs := &BlockSet{Blocks: make(map[string]BlockInfo)}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines[1:] { // Skip mode line
		if line == "" || strings.Contains(line, ".qtpl:") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue
		}

		blockID := parts[0]
		statements, _ := strconv.Atoi(parts[1])
		count, _ := strconv.Atoi(parts[2])

		// If block already exists, merge (keep max coverage)
		if existing, ok := bs.Blocks[blockID]; ok {
			bs.Blocks[blockID] = BlockInfo{
				Statements: existing.Statements,
				Covered:    existing.Covered || count > 0,
			}
		} else {
			bs.Blocks[blockID] = BlockInfo{
				Statements: statements,
				Covered:    count > 0,
			}
		}
	}

	return bs, nil
}

// Clone creates a deep copy of the BlockSet.
func (bs *BlockSet) Clone() *BlockSet {
	clone := &BlockSet{Blocks: make(map[string]BlockInfo, len(bs.Blocks))}
	for k, v := range bs.Blocks {
		clone.Blocks[k] = v
	}
	return clone
}

// Merge combines another BlockSet into this one (union of coverage).
func (bs *BlockSet) Merge(other *BlockSet) {
	for blockID, info := range other.Blocks {
		if existing, ok := bs.Blocks[blockID]; ok {
			bs.Blocks[blockID] = BlockInfo{
				Statements: existing.Statements,
				Covered:    existing.Covered || info.Covered,
			}
		} else {
			bs.Blocks[blockID] = info
		}
	}
}

// CoveredStatements returns the number of covered statements.
func (bs *BlockSet) CoveredStatements() int {
	count := 0
	for _, info := range bs.Blocks {
		if info.Covered {
			count += info.Statements
		}
	}
	return count
}

// TotalStatements returns the total number of statements.
func (bs *BlockSet) TotalStatements() int {
	count := 0
	for _, info := range bs.Blocks {
		count += info.Statements
	}
	return count
}

// CoveragePercent returns the overall coverage percentage.
func (bs *BlockSet) CoveragePercent() float64 {
	total := bs.TotalStatements()
	if total == 0 {
		return 0
	}
	return float64(bs.CoveredStatements()) * 100.0 / float64(total)
}

// NewBlocksFrom returns block IDs that are covered in other but not in bs.
func (bs *BlockSet) NewBlocksFrom(other *BlockSet) []string {
	var newBlocks []string
	for blockID, info := range other.Blocks {
		if !info.Covered {
			continue
		}
		if existing, ok := bs.Blocks[blockID]; !ok || !existing.Covered {
			newBlocks = append(newBlocks, blockID)
		}
	}
	return newBlocks
}

// CountNewStatements returns the number of new statements that would be covered
// if other's coverage was merged into this BlockSet.
func (bs *BlockSet) CountNewStatements(other *BlockSet) int {
	count := 0
	for blockID, info := range other.Blocks {
		if !info.Covered {
			continue
		}
		if existing, ok := bs.Blocks[blockID]; !ok || !existing.Covered {
			count += info.Statements
		}
	}
	return count
}
