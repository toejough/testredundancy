// Package analysis provides the core redundancy detection algorithm.
package analysis

// TestCoverage represents a test's coverage data.
type TestCoverage struct {
	TestName string
	// Maps function name to coverage percentage (0-100)
	Coverage map[string]float64
}

// SelectionResult contains the results of the minimal set selection.
type SelectionResult struct {
	KeptTests      []string
	RedundantTests []string
}

// SelectMinimalSet uses a greedy algorithm to find the minimal set of tests
// that maintain coverage above the threshold for all target functions.
// Baseline tests are always included.
func SelectMinimalSet(
	allCoverage []TestCoverage,
	baselineTests map[string]bool,
	targetFunctions map[string]bool,
	threshold float64,
) SelectionResult {
	if len(allCoverage) == 0 || len(targetFunctions) == 0 {
		return SelectionResult{}
	}

	// Build a map for quick lookup
	coverageByTest := make(map[string]map[string]float64)
	for _, tc := range allCoverage {
		coverageByTest[tc.TestName] = tc.Coverage
	}

	// Track current coverage level for each target function
	currentCoverage := make(map[string]float64)
	for fn := range targetFunctions {
		currentCoverage[fn] = 0
	}

	// Track which tests are kept
	keptTests := make(map[string]bool)
	var keptOrder []string

	// Helper to merge coverage (take max for each function)
	mergeCoverage := func(testName string) {
		cov, ok := coverageByTest[testName]
		if !ok {
			return
		}

		for fn, newCov := range cov {
			if _, isTarget := targetFunctions[fn]; isTarget {
				if newCov > currentCoverage[fn] {
					currentCoverage[fn] = newCov
				}
			}
		}
	}

	// Count how many target functions this test improves
	countImprovements := func(testName string) int {
		cov, ok := coverageByTest[testName]
		if !ok {
			return 0
		}

		count := 0

		for fn, newCov := range cov {
			if _, isTarget := targetFunctions[fn]; !isTarget {
				continue
			}

			// Count as improvement if it increases coverage
			if newCov > currentCoverage[fn] {
				count++
			}
		}

		return count
	}

	// Phase 1: Add baseline tests first (they're always kept)
	for _, tc := range allCoverage {
		if baselineTests[tc.TestName] {
			keptTests[tc.TestName] = true
			keptOrder = append(keptOrder, tc.TestName)
			mergeCoverage(tc.TestName)
		}
	}

	// Phase 2: Greedily add tests that provide the most improvement
	for {
		var bestTest string
		bestImprovements := 0

		for _, tc := range allCoverage {
			if keptTests[tc.TestName] {
				continue
			}

			improvements := countImprovements(tc.TestName)
			if improvements > bestImprovements {
				bestImprovements = improvements
				bestTest = tc.TestName
			}
		}

		// No more improvements possible
		if bestImprovements == 0 {
			break
		}

		keptTests[bestTest] = true
		keptOrder = append(keptOrder, bestTest)
		mergeCoverage(bestTest)
	}

	// Identify redundant tests
	var redundant []string

	for _, tc := range allCoverage {
		if !keptTests[tc.TestName] {
			redundant = append(redundant, tc.TestName)
		}
	}

	return SelectionResult{
		KeptTests:      keptOrder,
		RedundantTests: redundant,
	}
}
