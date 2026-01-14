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
// Baseline tests are preferred, then non-baseline tests are added greedily.
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

	// Track which functions still need coverage (below threshold)
	remainingGaps := make(map[string]bool)
	for fn := range targetFunctions {
		remainingGaps[fn] = true
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

	// Update remaining gaps after adding a test
	updateGaps := func() {
		for fn := range remainingGaps {
			if currentCoverage[fn] >= threshold {
				delete(remainingGaps, fn)
			}
		}
	}

	// Count how many gaps (functions below threshold) this test helps fill
	countGapImprovements := func(testName string) int {
		cov, ok := coverageByTest[testName]
		if !ok {
			return 0
		}

		count := 0

		for fn := range remainingGaps {
			// Count as improvement if this test increases coverage for a gap
			if newCov, hasCov := cov[fn]; hasCov && newCov > currentCoverage[fn] {
				count++
			}
		}

		return count
	}

	// Separate baseline and non-baseline tests
	var baselineTestList []TestCoverage
	var nonBaselineTestList []TestCoverage

	for _, tc := range allCoverage {
		if baselineTests[tc.TestName] {
			baselineTestList = append(baselineTestList, tc)
		} else {
			nonBaselineTestList = append(nonBaselineTestList, tc)
		}
	}

	// Keep adding tests until no gaps remain
	for len(remainingGaps) > 0 {
		var bestTest string
		bestImprovements := 0

		// First try baseline tests
		for _, tc := range baselineTestList {
			if keptTests[tc.TestName] {
				continue
			}

			improvements := countGapImprovements(tc.TestName)
			if improvements > bestImprovements {
				bestImprovements = improvements
				bestTest = tc.TestName
			}
		}

		// If no baseline test helps, try non-baseline tests
		if bestImprovements == 0 {
			for _, tc := range nonBaselineTestList {
				if keptTests[tc.TestName] {
					continue
				}

				improvements := countGapImprovements(tc.TestName)
				if improvements > bestImprovements {
					bestImprovements = improvements
					bestTest = tc.TestName
				}
			}
		}

		// No test can fill any remaining gaps
		if bestImprovements == 0 {
			break
		}

		keptTests[bestTest] = true
		keptOrder = append(keptOrder, bestTest)
		mergeCoverage(bestTest)
		updateGaps()
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
