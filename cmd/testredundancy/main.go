// Command testredundancy finds redundant tests based on coverage analysis.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/toejough/testredundancy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command line args
	// Usage: testredundancy [--baseline pkg1,pkg2,...] [--threshold N] [--coverpkg pkgs] <package>
	args := os.Args[1:]

	config := testredundancy.Config{
		CoverageThreshold: 80.0,
		PackageToAnalyze:  "./...",
		CoveragePackages:  "./...",
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--baseline":
			if i+1 >= len(args) {
				return fmt.Errorf("--baseline requires an argument")
			}
			i++
			for _, pkg := range strings.Split(args[i], ",") {
				pkg = strings.TrimSpace(pkg)
				if pkg != "" {
					config.BaselineTests = append(config.BaselineTests, testredundancy.BaselineTestSpec{Package: pkg})
				}
			}
		case "--threshold":
			if i+1 >= len(args) {
				return fmt.Errorf("--threshold requires an argument")
			}
			i++
			t, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return fmt.Errorf("invalid threshold: %w", err)
			}
			config.CoverageThreshold = t
		case "--coverpkg":
			if i+1 >= len(args) {
				return fmt.Errorf("--coverpkg requires an argument")
			}
			i++
			config.CoveragePackages = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag: %s", args[i])
			}
			config.PackageToAnalyze = args[i]
		}
	}

	return testredundancy.Find(config)
}
