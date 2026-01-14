// Package coverage provides utilities for parsing and manipulating Go coverage files.
package coverage

import (
	"fmt"
	"strconv"
	"strings"
)

// Block represents a single coverage block from a coverage file.
type Block struct {
	File       string
	StartLine  int
	StartCol   int
	EndLine    int
	EndCol     int
	Statements int
	Count      int
}

// ParseBlockID parses a block ID like "file.go:10.5,20.15" into its components.
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

// ParseBlock parses a coverage line like "file.go:10.5,20.15 3 1" into a Block.
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

// FormatBlock formats a Block as a coverage line.
func FormatBlock(b Block) string {
	return fmt.Sprintf("%s:%d.%d,%d.%d %d %d",
		b.File, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.Statements, b.Count)
}

// MergeBlocks merges duplicate coverage blocks by summing their counts.
// Input is the content of a coverage file, output is the merged content.
func MergeBlocks(content string) (string, error) {
	if content == "" {
		return "", nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", nil
	}

	// First line is mode
	modeLine := lines[0]

	// Merge blocks by key (file:start,end statements)
	blocks := make(map[string]Block)

	for _, line := range lines[1:] {
		if line == "" {
			continue
		}

		block, err := ParseBlock(line)
		if err != nil {
			continue
		}

		key := fmt.Sprintf("%s:%d.%d,%d.%d %d",
			block.File, block.StartLine, block.StartCol,
			block.EndLine, block.EndCol, block.Statements)

		if existing, ok := blocks[key]; ok {
			existing.Count += block.Count
			blocks[key] = existing
		} else {
			blocks[key] = block
		}
	}

	// Write merged blocks
	var result strings.Builder

	result.WriteString(modeLine)
	result.WriteString("\n")

	for _, block := range blocks {
		result.WriteString(FormatBlock(block))
		result.WriteString("\n")
	}

	return result.String(), nil
}

// FilterQtpl removes .qtpl entries from coverage content.
// These are typically generated template files that shouldn't be counted.
func FilterQtpl(content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("empty coverage content")
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("empty coverage content")
	}

	// Keep mode line, filter out .qtpl entries
	filtered := []string{lines[0]} // mode line

	for _, line := range lines[1:] {
		if line == "" || strings.Contains(line, ".qtpl:") {
			continue
		}

		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n"), nil
}
