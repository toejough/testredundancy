// Package main provides the testredundancy CLI.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/toejough/targ"
	"github.com/toejough/testredundancy"
)

// find defines the "find" subcommand arguments.
type find struct {
	Package   string `targ:"positional,required,desc=package pattern to analyze (e.g. ./...)"`
	Baseline  string `targ:"flag,desc=comma-separated baseline packages (always kept)"`
	Threshold string `targ:"flag,desc=coverage threshold percentage (0-100)"`
	CoverPkg  string `targ:"flag,name=coverpkg,desc=packages to measure coverage for"`
}

// Run is required by targ but not used - parsing only.
func (c *find) Run() { _ = c }

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var parsed find

	result, err := targ.Execute(args, &parsed)
	if err != nil {
		if result.Output != "" {
			fmt.Fprintln(os.Stderr, result.Output)
		}

		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Parse threshold
	var threshold float64 = 80.0 // default

	if parsed.Threshold != "" {
		var parseErr error

		threshold, parseErr = strconv.ParseFloat(parsed.Threshold, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid threshold value: %w", parseErr)
		}
	}

	// Build config from args
	config := testredundancy.Config{
		PackageToAnalyze:  parsed.Package,
		CoverageThreshold: threshold,
		CoveragePackages:  parsed.CoverPkg,
	}

	// Parse baseline specs
	if parsed.Baseline != "" {
		for _, pkg := range strings.Split(parsed.Baseline, ",") {
			pkg = strings.TrimSpace(pkg)
			if pkg != "" {
				config.BaselineTests = append(config.BaselineTests, testredundancy.BaselineTestSpec{
					Package: pkg,
				})
			}
		}
	}

	// For now, just print the config (full analysis not yet implemented)
	fmt.Println("Test Redundancy Analysis")
	fmt.Println("========================")
	fmt.Printf("Package to analyze: %s\n", config.PackageToAnalyze)
	fmt.Printf("Coverage threshold: %.1f%%\n", config.CoverageThreshold)

	if config.CoveragePackages != "" {
		fmt.Printf("Coverage packages:  %s\n", config.CoveragePackages)
	}

	if len(config.BaselineTests) > 0 {
		fmt.Printf("Baseline packages:  %d\n", len(config.BaselineTests))

		for _, spec := range config.BaselineTests {
			fmt.Printf("  - %s\n", spec.Package)
		}
	}

	fmt.Println()
	fmt.Println("Note: Full analysis not yet implemented.")
	fmt.Println("Use the testredundancy package API directly for now.")

	return nil
}
