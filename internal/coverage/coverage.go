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
