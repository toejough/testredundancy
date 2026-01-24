// Package testredundancy finds redundant tests based on coverage analysis.
package testredundancy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/toejough/testredundancy/internal/coverage"
	"github.com/toejough/testredundancy/internal/discovery"
	executil "github.com/toejough/testredundancy/internal/exec"
)

// BaselineTestSpec specifies a baseline test for redundancy analysis.
type BaselineTestSpec struct {
	Package     string // Package path (e.g., "./impgen/run" or "./UAT/...")
	TestPattern string // Test name pattern for -run flag (empty string runs all tests in package)
}

// Config configures the redundant test analysis.
type Config struct {
	BaselineTests     []BaselineTestSpec // Tests that form the baseline coverage
	CoverageThreshold float64            // Percentage threshold (e.g., 80.0 for 80%)
	PackageToAnalyze  string             // Package containing tests to analyze (e.g., "./impgen/run")
	CoveragePackages  string             // Packages to measure coverage for (e.g., "./impgen/...,./imptest/...")
}

// Find identifies unit tests that don't provide unique coverage beyond baseline tests.
// This generic version can be used in any repository by providing appropriate configuration.
func Find(config Config) error {
	fmt.Println("Finding redundant tests...")
	fmt.Println()

	// Default to ./... if not specified
	coverpkg := config.CoveragePackages
	if coverpkg == "" {
		coverpkg = "./..."
	}

	// Step 1: Identify baseline tests (preferred tests)
	fmt.Println("Step 1: Identifying baseline tests...")
	baselineTestSet := make(map[string]bool)    // key: "pkg:TestName" for exact matches
	baselinePatterns := make(map[string]string) // key: "pkg" -> pattern prefix

	for _, spec := range config.BaselineTests {
		if spec.TestPattern != "" {
			// Resolve package path to full module path for consistent matching
			fullPkg, err := executil.Output(context.Background(), "go", "list", spec.Package)
			if err != nil {
				return fmt.Errorf("failed to resolve package %s: %w", spec.Package, err)
			}

			fullPkg = strings.TrimSpace(fullPkg)
			// Store pattern for prefix matching
			baselinePatterns[fullPkg] = spec.TestPattern
		} else {
			// List all test functions in package
			pkgTests, err := discovery.ListTests(spec.Package)
			if err != nil {
				fmt.Printf("  Warning: couldn't list tests in %s: %v\n", spec.Package, err)
			} else {
				for _, t := range pkgTests {
					baselineTestSet[t.QualifiedName()] = true
				}
			}
		}
	}

	fmt.Printf("  Identified %d baseline test patterns, %d exact baseline tests\n", len(baselinePatterns), len(baselineTestSet))

	// Step 2: List all tests
	fmt.Println("\nStep 2: Listing all tests...")

	allTests, err := discovery.ListTests(config.PackageToAnalyze)
	if err != nil {
		return fmt.Errorf("failed to list tests: %w", err)
	}

	// Separate into baseline and non-baseline
	var baselineTests []discovery.TestInfo
	var nonBaselineTests []discovery.TestInfo

	// Helper to check if a test matches baseline criteria
	isBaseline := func(t discovery.TestInfo) bool {
		// Check exact match
		if baselineTestSet[t.QualifiedName()] {
			return true
		}
		// Check pattern prefix match
		if pattern, ok := baselinePatterns[t.Pkg]; ok {
			if strings.HasPrefix(t.Name, pattern) {
				return true
			}
		}
		return false
	}

	for _, t := range allTests {
		if isBaseline(t) {
			baselineTests = append(baselineTests, t)
		} else {
			nonBaselineTests = append(nonBaselineTests, t)
		}
	}

	fmt.Printf("  Found %d baseline tests, %d non-baseline tests (%d total)\n",
		len(baselineTests), len(nonBaselineTests), len(allTests))

	// Step 3: Run each test individually to collect coverage
	fmt.Println("\nStep 3: Running each test individually to collect coverage...")

	// Combine all tests
	allTestsToRun := append(baselineTests, nonBaselineTests...)

	// Detect which tests are marked with t.Parallel()
	fmt.Println("  Detecting parallel-safe tests...")

	parallelTests := discovery.DetectParallelTests(allTestsToRun)
	fmt.Printf("  Found %d parallel-safe tests, %d serial tests\n",
		len(parallelTests), len(allTestsToRun)-len(parallelTests))

	testCoverageFiles := make(map[string]string)
	var allTestOrder []discovery.TestInfo

	// Helper to run a single test and collect coverage
	runSingleTest := func(test discovery.TestInfo) bool {
		coverFile := fmt.Sprintf("cov_%s_%s.out", executil.Sanitize(filepath.Base(test.Pkg)), test.Name)
		coverFileRaw := coverFile + ".raw"

		testErr := executil.RunQuietCoverage("go", "test", "-count=1", "-coverprofile="+coverFileRaw, "-coverpkg="+coverpkg,
			"-run", "^"+test.Name+"$", test.Pkg)

		if testErr != nil {
			return false
		}

		err := coverage.FilterQtpl(coverFileRaw, coverFile)
		if err != nil {
			os.Remove(coverFileRaw)

			return false
		}

		os.Remove(coverFileRaw)
		testCoverageFiles[test.QualifiedName()] = coverFile
		allTestOrder = append(allTestOrder, test)

		return true
	}

	// Separate tests into parallel-safe and serial
	var serialTests []discovery.TestInfo
	var parallelSafeTests []discovery.TestInfo

	for _, test := range allTestsToRun {
		if parallelTests[test.QualifiedName()] {
			parallelSafeTests = append(parallelSafeTests, test)
		} else {
			serialTests = append(serialTests, test)
		}
	}

	// Run serial tests first (sequentially)
	if len(serialTests) > 0 {
		fmt.Printf("  Running %d serial tests sequentially...\n", len(serialTests))

		for i, test := range serialTests {
			fmt.Printf("    [%d/%d] %s... ", i+1, len(serialTests), test.QualifiedName())

			if runSingleTest(test) {
				fmt.Printf("OK\n")
			} else {
				fmt.Printf("FAILED\n")
			}
		}
	}

	// Run parallel-safe tests concurrently
	if len(parallelSafeTests) > 0 {
		fmt.Printf("  Running %d parallel-safe tests concurrently...\n", len(parallelSafeTests))

		var testCoverageFilesMu sync.Mutex
		var allTestOrderMu sync.Mutex

		numWorkers := runtime.NumCPU()
		sem := make(chan struct{}, numWorkers)
		var wg sync.WaitGroup
		var completed int32

		for _, test := range parallelSafeTests {
			wg.Add(1)

			go func(test discovery.TestInfo) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				coverFile := fmt.Sprintf("cov_%s_%s.out", executil.Sanitize(filepath.Base(test.Pkg)), test.Name)
				coverFileRaw := coverFile + ".raw"

				testErr := executil.RunQuietCoverage("go", "test", "-count=1", "-coverprofile="+coverFileRaw, "-coverpkg="+coverpkg,
					"-run", "^"+test.Name+"$", test.Pkg)

				current := atomic.AddInt32(&completed, 1)

				if testErr != nil {
					fmt.Printf("    [%d/%d] %s... FAILED\n", current, len(parallelSafeTests), test.QualifiedName())

					return
				}

				err := coverage.FilterQtpl(coverFileRaw, coverFile)
				if err != nil {
					fmt.Printf("    [%d/%d] %s... FAILED (filter)\n", current, len(parallelSafeTests), test.QualifiedName())
					os.Remove(coverFileRaw)

					return
				}

				os.Remove(coverFileRaw)

				testCoverageFilesMu.Lock()
				testCoverageFiles[test.QualifiedName()] = coverFile
				testCoverageFilesMu.Unlock()

				allTestOrderMu.Lock()
				allTestOrder = append(allTestOrder, test)
				allTestOrderMu.Unlock()

				fmt.Printf("    [%d/%d] %s... OK\n", current, len(parallelSafeTests), test.QualifiedName())
			}(test)
		}

		wg.Wait()
	}

	// Step 4: Parse coverage files into memory and compute total function coverage
	fmt.Println("\nStep 4: Parsing coverage files and computing function coverage...")

	// Parse all coverage files into BlockSets (in-memory)
	testBlockSets := make(map[string]*coverage.BlockSet)

	for qName, coverFile := range testCoverageFiles {
		bs, err := coverage.ParseFileToBlockSet(coverFile)
		if err != nil {
			fmt.Printf("  Warning: failed to parse %s: %v\n", coverFile, err)
			continue
		}
		testBlockSets[qName] = bs
	}

	if len(testBlockSets) == 0 {
		return fmt.Errorf("no tests ran successfully")
	}

	fmt.Printf("  Parsed %d coverage files into memory\n", len(testBlockSets))

	// Compute total coverage by merging all blocks
	totalBlockSet := &coverage.BlockSet{Blocks: make(map[string]coverage.BlockInfo)}
	for _, bs := range testBlockSets {
		totalBlockSet.Merge(bs)
	}

	// Write merged coverage to temp file and get function coverage
	totalCoverageFile := "total_coverage_temp.out"
	if err := coverage.WriteBlockSetToFile(totalBlockSet, totalCoverageFile); err != nil {
		return fmt.Errorf("failed to write total coverage: %w", err)
	}

	totalFuncCoverage, err := coverage.GetAllFunctionsCoverage(totalCoverageFile)
	os.Remove(totalCoverageFile)
	if err != nil {
		return fmt.Errorf("failed to get function coverage: %w", err)
	}

	// Identify target functions (those that reach threshold with all tests)
	targetFuncs := make(map[string]bool)
	for fn, cov := range totalFuncCoverage {
		if cov >= config.CoverageThreshold {
			targetFuncs[fn] = true
		}
	}

	fmt.Printf("  Target: %d functions at %.0f%%+ (with all tests)\n", len(targetFuncs), config.CoverageThreshold)

	// Step 5: Greedy addition using block-level coverage (in-memory)
	// This properly handles the additive nature of coverage
	fmt.Println("\nStep 5: Building minimal test set from zero (preferring baseline tests)...")
	fmt.Printf("  %-80s %6s   %s\n", "TEST", "NEW", "DECISION")
	fmt.Printf("  %-80s %6s   %s\n", strings.Repeat("-", 80), "------", "--------")

	type testResult struct {
		name       string
		pkg        string
		isBaseline bool
		gapsFilled int
	}

	var keptTests []testResult
	keptTestFiles := []string{}
	keptTestSet := make(map[string]bool)

	// Track current merged coverage (starts empty)
	currentCoverage := &coverage.BlockSet{Blocks: make(map[string]coverage.BlockInfo)}

	// Helper to find best test from a pool (in-memory, no I/O)
	findBestTest := func(pool []discovery.TestInfo) (discovery.TestInfo, int) {
		var bestTest discovery.TestInfo
		bestNewStatements := 0

		for _, test := range pool {
			qName := test.QualifiedName()
			if keptTestSet[qName] {
				continue
			}

			testBS := testBlockSets[qName]
			if testBS == nil {
				continue
			}

			// Count new statements this test would contribute
			newStatements := currentCoverage.CountNewStatements(testBS)
			if newStatements > bestNewStatements {
				bestNewStatements = newStatements
				bestTest = test
			}
		}

		return bestTest, bestNewStatements
	}

	// Keep adding tests until coverage stops improving
	for {
		// First try baseline tests
		bestTest, newStatements := findBestTest(baselineTests)
		isBaseline := true

		// If no baseline test adds coverage, try non-baseline tests
		if newStatements == 0 {
			bestTest, newStatements = findBestTest(nonBaselineTests)
			isBaseline = false
		}

		if newStatements == 0 {
			// No test can add any new coverage
			break
		}

		// Add the best test
		qName := bestTest.QualifiedName()
		marker := ""
		if isBaseline {
			marker = " (baseline)"
		}

		fmt.Printf("  %-80s %6d   KEEP%s\n", qName, newStatements, marker)

		keptTests = append(keptTests, testResult{
			name:       bestTest.Name,
			pkg:        bestTest.Pkg,
			isBaseline: isBaseline,
			gapsFilled: newStatements,
		})
		keptTestSet[qName] = true
		keptTestFiles = append(keptTestFiles, testCoverageFiles[qName])

		// Merge this test's coverage into current
		currentCoverage.Merge(testBlockSets[qName])
	}

	// Mark remaining tests as redundant
	var redundantBaselineTests []testResult
	var redundantNonBaselineTests []testResult

	for _, test := range allTestOrder {
		qName := test.QualifiedName()
		if !keptTestSet[qName] {
			testIsBaseline := isBaseline(test)
			marker := ""
			if testIsBaseline {
				marker = " (baseline)"
			}

			fmt.Printf("  %-80s %6d   REDUNDANT%s\n", qName, 0, marker)

			result := testResult{
				name:       test.Name,
				pkg:        test.Pkg,
				isBaseline: testIsBaseline,
			}

			if testIsBaseline {
				redundantBaselineTests = append(redundantBaselineTests, result)
			} else {
				redundantNonBaselineTests = append(redundantNonBaselineTests, result)
			}
		}
	}

	// Validation
	fmt.Println("\nStep 6: Validating final coverage...")

	if len(keptTestFiles) == 0 {
		fmt.Println("  WARNING: No tests kept - validation skipped")
	} else {
		// Write current merged coverage to temp file and compute function coverage
		keptCoverageFile := "kept_coverage_temp.out"
		if err := coverage.WriteBlockSetToFile(currentCoverage, keptCoverageFile); err != nil {
			fmt.Printf("  VALIDATION ERROR: failed to write coverage: %v\n", err)
		} else {
			keptFuncCoverage, err := coverage.GetAllFunctionsCoverage(keptCoverageFile)
			os.Remove(keptCoverageFile)

			if err != nil {
				fmt.Printf("  VALIDATION ERROR: failed to compute function coverage: %v\n", err)
			} else {
				// Count how many target functions are now at threshold
				coveredFuncs := 0
				for fn := range targetFuncs {
					if keptFuncCoverage[fn] >= config.CoverageThreshold {
						coveredFuncs++
					}
				}

				if coveredFuncs < len(targetFuncs) {
					fmt.Printf("  VALIDATION WARNING: Only %d/%d target functions at %.0f%%+ coverage\n",
						coveredFuncs, len(targetFuncs), config.CoverageThreshold)
				} else {
					fmt.Printf("  VALIDATION PASSED: All %d target functions maintain %.0f%%+ coverage\n",
						len(targetFuncs), config.CoverageThreshold)
				}
			}
		}
	}

	// Clean up
	for _, f := range testCoverageFiles {
		os.Remove(f)
	}

	// Report results
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("RESULTS")
	fmt.Println("=" + strings.Repeat("=", 79))

	// Count kept by type
	var keptBaseline, keptNonBaseline int

	for _, t := range keptTests {
		if t.isBaseline {
			keptBaseline++
		} else {
			keptNonBaseline++
		}
	}

	fmt.Printf("\nTests that must be kept (%d total: %d baseline, %d non-baseline):\n",
		len(keptTests), keptBaseline, keptNonBaseline)
	fmt.Printf("  %-80s %6s   %s\n", "TEST", "FILLS", "TYPE")
	fmt.Printf("  %-80s %6s   %s\n", strings.Repeat("-", 80), "------", "--------")

	for _, test := range keptTests {
		qName := test.pkg + ":" + test.name
		typeStr := "unit"
		if test.isBaseline {
			typeStr = "baseline"
		}

		fmt.Printf("  %-80s %6d   %s\n", qName, test.gapsFilled, typeStr)
	}

	// Trimming report - redundant baseline tests
	fmt.Printf("\nBaseline tests that could be trimmed (%d):\n", len(redundantBaselineTests))
	fmt.Printf("  %-80s\n", "TEST")
	fmt.Printf("  %-80s\n", strings.Repeat("-", 80))

	sort.Slice(redundantBaselineTests, func(i, j int) bool {
		if redundantBaselineTests[i].pkg != redundantBaselineTests[j].pkg {
			return redundantBaselineTests[i].pkg < redundantBaselineTests[j].pkg
		}

		return redundantBaselineTests[i].name < redundantBaselineTests[j].name
	})

	for _, test := range redundantBaselineTests {
		qName := test.pkg + ":" + test.name
		fmt.Printf("  %-80s\n", qName)
	}

	// Redundant non-baseline tests
	fmt.Printf("\nRedundant non-baseline tests (%d):\n", len(redundantNonBaselineTests))
	fmt.Printf("  %-80s\n", "TEST")
	fmt.Printf("  %-80s\n", strings.Repeat("-", 80))

	sort.Slice(redundantNonBaselineTests, func(i, j int) bool {
		if redundantNonBaselineTests[i].pkg != redundantNonBaselineTests[j].pkg {
			return redundantNonBaselineTests[i].pkg < redundantNonBaselineTests[j].pkg
		}

		return redundantNonBaselineTests[i].name < redundantNonBaselineTests[j].name
	})

	for _, test := range redundantNonBaselineTests {
		qName := test.pkg + ":" + test.name
		fmt.Printf("  %-80s\n", qName)
	}

	fmt.Println()

	return nil
}
