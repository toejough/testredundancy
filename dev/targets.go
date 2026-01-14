// Package dev provides build and development targets for testredundancy.
package dev

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/toejough/targ"
)

// Check runs all checks: test, lint, and build.
type Check struct{}

// Run executes all checks.
func (c Check) Run(ctx context.Context) error {
	return targ.Deps(
		Test{},
		Lint{},
		Build{},
		targ.WithContext(ctx),
	)
}

// Test runs all tests.
type Test struct{}

// Run executes tests.
func (t Test) Run() error {
	fmt.Println("Running tests...")

	cmd := exec.Command("go", "test", "-race", "-cover", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Lint runs the linter.
type Lint struct{}

// Run executes the linter.
func (l Lint) Run() error {
	fmt.Println("Running linter...")

	cmd := exec.Command("golangci-lint", "run", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Build builds the CLI binary.
type Build struct{}

// Run executes the build.
func (b Build) Run() error {
	fmt.Println("Building...")

	cmd := exec.Command("go", "build", "-o", "bin/testredundancy", "./cmd/testredundancy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
