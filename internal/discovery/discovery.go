// Package discovery provides test discovery utilities for Go projects.
package discovery

import (
	"go/ast"
	"strings"
)

// TestInfo represents a test function with its package.
type TestInfo struct {
	Pkg  string
	Name string
}

// QualifiedName returns the package-qualified test name (pkg:TestName).
func (t TestInfo) QualifiedName() string {
	return t.Pkg + ":" + t.Name
}

// HasParallelCall checks if a function body contains a t.Parallel() call.
func HasParallelCall(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}

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

// ParseTestOutput parses the output of "go test -list ." and returns test info.
func ParseTestOutput(pkg string, output string) []TestInfo {
	var tests []TestInfo

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Test") {
			tests = append(tests, TestInfo{Pkg: pkg, Name: line})
		}
	}

	return tests
}
