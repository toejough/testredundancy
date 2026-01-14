// Package testredundancy identifies redundant tests in Go projects.
//
// It analyzes test coverage to find the minimal set of tests needed to
// maintain a coverage threshold, identifying tests that don't contribute
// unique coverage beyond a baseline test set.
package testredundancy

// BaselineTestSpec specifies tests that should always be kept.
type BaselineTestSpec struct {
	// Package is the package path (e.g., "./uat/...")
	Package string
	// TestPattern is an optional regex pattern to match specific tests.
	// If empty, all tests in the package are considered baseline.
	TestPattern string
}

// Config holds the configuration for finding redundant tests.
type Config struct {
	// BaselineTests are tests that should always be kept.
	BaselineTests []BaselineTestSpec
	// CoverageThreshold is the minimum coverage percentage (0-100) to maintain.
	CoverageThreshold float64
	// PackageToAnalyze is the package pattern to analyze (e.g., "./...").
	PackageToAnalyze string
	// CoveragePackages specifies which packages to measure coverage for.
	// Defaults to "./..." if empty.
	CoveragePackages string
}

// Result contains the analysis results.
type Result struct {
	// KeptTests are the tests that should be kept (including baseline).
	KeptTests []string
	// RedundantTests are tests that can be removed without losing coverage.
	RedundantTests []string
}
