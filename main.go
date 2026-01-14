// Package main provides the testredundancy CLI.
package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/toejough/testredundancy/internal/coverage"
	executil "github.com/toejough/testredundancy/internal/exec"
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
	
	config := RedundancyConfig{
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
					config.BaselineTests = append(config.BaselineTests, BaselineTestSpec{Package: pkg})
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
	
	return findRedundantTestsWithConfig(config)
}

// BaselineTestSpec specifies a baseline test for redundancy analysis.
type BaselineTestSpec struct {
	Package     string // Package path (e.g., "./impgen/run" or "./UAT/...")
	TestPattern string // Test name pattern for -run flag (empty string runs all tests in package)
}

// RedundancyConfig configures the redundant test analysis.
type RedundancyConfig struct {
	BaselineTests     []BaselineTestSpec // Tests that form the baseline coverage
	CoverageThreshold float64            // Percentage threshold (e.g., 80.0 for 80%)
	PackageToAnalyze  string             // Package containing tests to analyze (e.g., "./impgen/run")
	CoveragePackages  string             // Packages to measure coverage for (e.g., "./impgen/...,./imptest/...")
}

type testInfo struct {
	pkg  string
	name string
}

// qualifiedName returns the package-qualified test name (pkg:TestName).
func (t testInfo) qualifiedName() string {
	return t.pkg + ":" + t.name
}

// detectParallelTests detects which tests are marked with t.Parallel().
// Returns a map of qualified test names (pkg:TestName) that are parallel-safe.
func detectParallelTests(tests []testInfo) map[string]bool {
	result := make(map[string]bool)

	// Group tests by package
	testsByPkg := make(map[string][]testInfo)
	for _, t := range tests {
		testsByPkg[t.pkg] = append(testsByPkg[t.pkg], t)
	}

	for pkg, pkgTests := range testsByPkg {
		// Get the directory for this package
		pkgDir, err := executil.Output(context.Background(), "go", "list", "-f", "{{.Dir}}", pkg)
		if err != nil {
			continue
		}

		pkgDir = strings.TrimSpace(pkgDir)

		// Find test files in this package
		testFiles, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
		if err != nil {
			continue
		}

		// Parse each test file and check for t.Parallel() calls
		fset := token.NewFileSet()

		for _, testFile := range testFiles {
			f, err := parser.ParseFile(fset, testFile, nil, parser.ParseComments)
			if err != nil {
				continue
			}

			// Find all test functions and check if they call t.Parallel()
			ast.Inspect(f, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Name == nil {
					return true
				}

				// Only consider Test* functions with *testing.T parameter
				if !strings.HasPrefix(fn.Name.Name, "Test") {
					return true
				}

				// Check if this function is one we care about
				qualifiedName := pkg + ":" + fn.Name.Name
				isRelevant := false

				for _, t := range pkgTests {
					if t.qualifiedName() == qualifiedName {
						isRelevant = true

						break
					}
				}

				if !isRelevant {
					return true
				}

				// Check if function body contains t.Parallel() call
				if fn.Body != nil && hasParallelCall(fn.Body) {
					result[qualifiedName] = true
				}

				return true
			})
		}
	}

	return result
}


// findRedundantTestsWithConfig identifies unit tests that don't provide unique coverage beyond baseline tests.
// This generic version can be used in any repository by providing appropriate configuration.
func findRedundantTestsWithConfig(config RedundancyConfig) error {
	fmt.Println("Finding redundant tests...")
	fmt.Println()

	// Default to ./... if not specified
	coverpkg := config.CoveragePackages
	if coverpkg == "" {
		coverpkg = "./..."
	}

	// Step 1: Identify baseline tests (preferred tests)
	fmt.Println("Step 1: Identifying baseline tests...")
	baselineTestSet := make(map[string]bool) // key: "pkg:TestName"

	for _, spec := range config.BaselineTests {
		if spec.TestPattern != "" {
			// Resolve package path to full module path for consistent matching
			fullPkg, err := executil.Output(context.Background(), "go", "list", spec.Package)
			if err != nil {
				return fmt.Errorf("failed to resolve package %s: %w", spec.Package, err)
			}

			baselineTestSet[strings.TrimSpace(fullPkg)+":"+spec.TestPattern] = true
		} else {
			// List all test functions in package
			pkgTests, err := listTestFunctionsWithPackages(spec.Package)
			if err != nil {
				fmt.Printf("  Warning: couldn't list tests in %s: %v\n", spec.Package, err)
			} else {
				for _, t := range pkgTests {
					baselineTestSet[t.qualifiedName()] = true
				}
			}
		}
	}

	fmt.Printf("  Identified %d baseline tests\n", len(baselineTestSet))

	// Step 2: List all tests
	fmt.Println("\nStep 2: Listing all tests...")

	allTests, err := listTestFunctionsWithPackages(config.PackageToAnalyze)
	if err != nil {
		return fmt.Errorf("failed to list tests: %w", err)
	}

	// Separate into baseline and non-baseline
	var baselineTests []testInfo
	var nonBaselineTests []testInfo

	for _, t := range allTests {
		if baselineTestSet[t.qualifiedName()] {
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

	parallelTests := detectParallelTests(allTestsToRun)
	fmt.Printf("  Found %d parallel-safe tests, %d serial tests\n",
		len(parallelTests), len(allTestsToRun)-len(parallelTests))

	testCoverageFiles := make(map[string]string)
	var allTestOrder []testInfo

	// Helper to run a single test and collect coverage
	runSingleTest := func(test testInfo) bool {
		coverFile := fmt.Sprintf("cov_%s_%s.out", executil.Sanitize(filepath.Base(test.pkg)), test.name)
		coverFileRaw := coverFile + ".raw"

		testErr := executil.RunQuietCoverage("go", "test", "-count=1", "-coverprofile="+coverFileRaw, "-coverpkg="+coverpkg,
			"-run", "^"+test.name+"$", test.pkg)

		if testErr != nil {
			return false
		}

		err := coverage.FilterQtpl(coverFileRaw, coverFile)
		if err != nil {
			os.Remove(coverFileRaw)

			return false
		}

		os.Remove(coverFileRaw)
		testCoverageFiles[test.qualifiedName()] = coverFile
		allTestOrder = append(allTestOrder, test)

		return true
	}

	// Separate tests into parallel-safe and serial
	var serialTests []testInfo
	var parallelSafeTests []testInfo

	for _, test := range allTestsToRun {
		if parallelTests[test.qualifiedName()] {
			parallelSafeTests = append(parallelSafeTests, test)
		} else {
			serialTests = append(serialTests, test)
		}
	}

	// Run serial tests first (sequentially)
	if len(serialTests) > 0 {
		fmt.Printf("  Running %d serial tests sequentially...\n", len(serialTests))

		for i, test := range serialTests {
			fmt.Printf("    [%d/%d] %s... ", i+1, len(serialTests), test.qualifiedName())

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

			go func(test testInfo) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				coverFile := fmt.Sprintf("cov_%s_%s.out", executil.Sanitize(filepath.Base(test.pkg)), test.name)
				coverFileRaw := coverFile + ".raw"

				testErr := executil.RunQuietCoverage("go", "test", "-count=1", "-coverprofile="+coverFileRaw, "-coverpkg="+coverpkg,
					"-run", "^"+test.name+"$", test.pkg)

				current := atomic.AddInt32(&completed, 1)

				if testErr != nil {
					fmt.Printf("    [%d/%d] %s... FAILED\n", current, len(parallelSafeTests), test.qualifiedName())

					return
				}

				err := coverage.FilterQtpl(coverFileRaw, coverFile)
				if err != nil {
					fmt.Printf("    [%d/%d] %s... FAILED (filter)\n", current, len(parallelSafeTests), test.qualifiedName())
					os.Remove(coverFileRaw)

					return
				}

				os.Remove(coverFileRaw)

				testCoverageFilesMu.Lock()
				testCoverageFiles[test.qualifiedName()] = coverFile
				testCoverageFilesMu.Unlock()

				allTestOrderMu.Lock()
				allTestOrder = append(allTestOrder, test)
				allTestOrderMu.Unlock()

				fmt.Printf("    [%d/%d] %s... OK\n", current, len(parallelSafeTests), test.qualifiedName())
			}(test)
		}

		wg.Wait()
	}

	// Step 4: Compute target coverage (all tests merged)
	fmt.Println("\nStep 4: Computing target coverage with all tests...")

	var allCoverageFiles []string
	for _, f := range testCoverageFiles {
		allCoverageFiles = append(allCoverageFiles, f)
	}

	if len(allCoverageFiles) == 0 {
		return fmt.Errorf("no tests ran successfully")
	}

	totalCoverageFile := "total_coverage.out"

	err = coverage.MergeFiles(allCoverageFiles, totalCoverageFile)
	if err != nil {
		return fmt.Errorf("failed to merge total coverage: %w", err)
	}

	targetCoverage, err := coverage.GetAllFunctionsCoverage(totalCoverageFile)
	if err != nil {
		return fmt.Errorf("failed to analyze target coverage: %w", err)
	}

	// Build target set: functions that reach threshold with all tests
	targetFuncs := make(map[string]bool)

	for fn, cov := range targetCoverage {
		if cov >= config.CoverageThreshold {
			targetFuncs[fn] = true
		}
	}

	fmt.Printf("  Target: %d functions at %.0f%%+ (with all tests)\n", len(targetFuncs), config.CoverageThreshold)

	os.Remove(totalCoverageFile)

	// Step 5: Greedy addition starting from 0 coverage
	fmt.Println("\nStep 5: Building minimal test set from zero (preferring baseline tests)...")
	fmt.Printf("  %-80s %6s   %s\n", "TEST", "FILLS", "DECISION")
	fmt.Printf("  %-80s %6s   %s\n", strings.Repeat("-", 80), "------", "--------")

	type testResult struct {
		name       string
		pkg        string
		isBaseline bool
		gapsFilled int
	}

	var keptTests []testResult
	keptTestFiles := []string{}

	// Track current coverage level for each target function (starts at 0)
	currentCoverage := make(map[string]float64)
	for fn := range targetFuncs {
		currentCoverage[fn] = 0
	}

	// Track which functions still need coverage (below threshold)
	remainingGaps := make(map[string]bool)
	for fn := range targetFuncs {
		remainingGaps[fn] = true
	}

	keptTestSet := make(map[string]bool)

	// Maintain a running "merged so far" file to avoid re-merging all kept files each time
	currentMergedFile := ""

	// Helper to evaluate a single test candidate
	// Returns list of functions where this test improves coverage (for functions still below threshold)
	evalCandidate := func(test testInfo, mergedSoFar string, gaps map[string]bool, currCov map[string]float64) []string {
		qName := test.qualifiedName()
		coverFile := testCoverageFiles[qName]

		if coverFile == "" {
			return nil
		}

		var improvements []string

		if mergedSoFar == "" {
			// First test - check its coverage directly
			testCov, covErr := coverage.GetAllFunctionsCoverage(coverFile)
			if covErr != nil {
				return nil
			}

			for fn := range gaps {
				// Count as improvement if this test provides any coverage for an unfilled gap
				if testCov[fn] > currCov[fn] {
					improvements = append(improvements, fn)
				}
			}
		} else {
			// Merge candidate with current merged coverage (just 2 files!)
			mergedFile := fmt.Sprintf("merged_%s.out", executil.Sanitize(qName))

			mergeErr := coverage.MergeFiles([]string{mergedSoFar, coverFile}, mergedFile)
			if mergeErr != nil {
				return nil
			}

			mergedCov, covErr := coverage.GetAllFunctionsCoverage(mergedFile)
			os.Remove(mergedFile)

			if covErr != nil {
				return nil
			}

			for fn := range gaps {
				// Count as improvement if merged coverage is better than current
				if mergedCov[fn] > currCov[fn] {
					improvements = append(improvements, fn)
				}
			}
		}

		return improvements
	}

	// Helper to find best test from a pool (parallelized)
	findBestTest := func(pool []testInfo) (testInfo, []string) {
		// Filter out already-kept tests
		var candidates []testInfo

		for _, test := range pool {
			if !keptTestSet[test.qualifiedName()] {
				candidates = append(candidates, test)
			}
		}

		if len(candidates) == 0 {
			return testInfo{}, nil
		}

		// Copy current state for parallel evaluation
		gapsCopy := make(map[string]bool)
		for fn := range remainingGaps {
			gapsCopy[fn] = true
		}

		covCopy := make(map[string]float64)
		for fn, cov := range currentCoverage {
			covCopy[fn] = cov
		}

		// Evaluate candidates in parallel
		type result struct {
			test         testInfo
			improvements []string
		}

		results := make([]result, len(candidates))
		var wg sync.WaitGroup
		// Use more workers for I/O-bound work (file merging + process spawning)
		sem := make(chan struct{}, runtime.NumCPU()*4)

		for i, test := range candidates {
			wg.Add(1)

			go func(idx int, t testInfo) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				improved := evalCandidate(t, currentMergedFile, gapsCopy, covCopy)
				results[idx] = result{test: t, improvements: improved}
			}(i, test)
		}

		wg.Wait()

		// Find best result (most improvements)
		var bestTest testInfo
		var bestImprovements []string

		for _, r := range results {
			if len(r.improvements) > len(bestImprovements) {
				bestTest = r.test
				bestImprovements = r.improvements
			}
		}

		return bestTest, bestImprovements
	}

	// Keep adding tests until no gaps remain
	for len(remainingGaps) > 0 {
		// First try baseline tests
		bestTest, bestGapsFilled := findBestTest(baselineTests)
		isBaseline := true

		// If no baseline test fills any gap, try non-baseline tests
		if len(bestGapsFilled) == 0 {
			bestTest, bestGapsFilled = findBestTest(nonBaselineTests)
			isBaseline = false
		}

		if len(bestGapsFilled) == 0 {
			// No test can fill any remaining gaps
			break
		}

		// Add the best test
		qName := bestTest.qualifiedName()
		marker := ""
		if isBaseline {
			marker = " (baseline)"
		}

		fmt.Printf("  %-80s %6d   KEEP%s\n", qName, len(bestGapsFilled), marker)

		keptTests = append(keptTests, testResult{
			name:       bestTest.name,
			pkg:        bestTest.pkg,
			isBaseline: isBaseline,
			gapsFilled: len(bestGapsFilled),
		})
		keptTestSet[qName] = true
		keptTestFiles = append(keptTestFiles, testCoverageFiles[qName])

		// Update the running merged coverage file
		newMergedFile := fmt.Sprintf("current_merged_%d.out", len(keptTestFiles))

		if currentMergedFile == "" {
			// First test - just copy its coverage file
			data, _ := os.ReadFile(testCoverageFiles[qName])
			_ = os.WriteFile(newMergedFile, data, 0o600)
		} else {
			// Merge with existing
			_ = coverage.MergeFiles([]string{currentMergedFile, testCoverageFiles[qName]}, newMergedFile)
			os.Remove(currentMergedFile)
		}

		currentMergedFile = newMergedFile

		// Update current coverage levels from new merged file
		newCoverage, err := coverage.GetAllFunctionsCoverage(newMergedFile)
		if err == nil {
			for fn := range targetFuncs {
				if newCov, ok := newCoverage[fn]; ok {
					currentCoverage[fn] = newCov
				}
			}
		}

		// Remove gaps only when they reach threshold
		for fn := range remainingGaps {
			if currentCoverage[fn] >= config.CoverageThreshold {
				delete(remainingGaps, fn)
			}
		}
	}

	// Clean up the final merged file
	if currentMergedFile != "" {
		os.Remove(currentMergedFile)
	}

	// Mark remaining tests as redundant
	var redundantBaselineTests []testResult
	var redundantNonBaselineTests []testResult

	for _, test := range allTestOrder {
		qName := test.qualifiedName()
		if !keptTestSet[qName] {
			isBaseline := baselineTestSet[qName]
			marker := ""
			if isBaseline {
				marker = " (baseline)"
			}

			fmt.Printf("  %-80s %6d   REDUNDANT%s\n", qName, 0, marker)

			result := testResult{
				name:       test.name,
				pkg:        test.pkg,
				isBaseline: isBaseline,
			}

			if isBaseline {
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
		finalMergedFile := "final_coverage.out"

		err = coverage.MergeFiles(keptTestFiles, finalMergedFile)
		if err != nil {
			return fmt.Errorf("failed to merge final coverage: %w", err)
		}

		finalCoverage, err := coverage.GetAllFunctionsCoverage(finalMergedFile)
		if err != nil {
			os.Remove(finalMergedFile)

			return fmt.Errorf("failed to analyze final coverage: %w", err)
		}

		os.Remove(finalMergedFile)

		var validationErrors []string

		for fn := range targetFuncs {
			if finalCoverage[fn] < config.CoverageThreshold {
				validationErrors = append(validationErrors,
					fmt.Sprintf("  %s: %.1f%% (target: %.0f%%)", fn, finalCoverage[fn], config.CoverageThreshold))
			}
		}

		if len(validationErrors) > 0 {
			fmt.Printf("  VALIDATION FAILED: %d functions dropped below threshold:\n", len(validationErrors))

			for _, e := range validationErrors {
				fmt.Println(e)
			}

			return fmt.Errorf("validation failed: %d functions dropped below coverage threshold", len(validationErrors))
		}

		fmt.Printf("  VALIDATION PASSED: All %d target functions maintain %.0f%%+ coverage\n",
			len(targetFuncs), config.CoverageThreshold)
	}

	// Report unfilled gaps
	if len(remainingGaps) > 0 {
		fmt.Printf("\n  WARNING: %d functions could not reach %.0f%% even with all tests:\n",
			len(remainingGaps), config.CoverageThreshold)

		for fn := range remainingGaps {
			fmt.Printf("    %s: %.1f%%\n", fn, targetCoverage[fn])
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

// hasParallelCall checks if a block statement contains a call to t.Parallel().
func hasParallelCall(body *ast.BlockStmt) bool {
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

// hasRelevantChanges returns true if the changeset contains files we care about.

// listTestFunctionsWithPackages lists all test functions with their packages.
func listTestFunctionsWithPackages(pkgPattern string) ([]testInfo, error) {
	// First, expand the package pattern to get actual packages
	listOut, err := executil.Output(context.Background(), "go", "list", pkgPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	var allTests []testInfo
	packages := strings.Split(strings.TrimSpace(listOut), "\n")

	for _, pkg := range packages {
		if pkg == "" {
			continue
		}

		out, err := executil.Output(context.Background(), "go", "test", "-list", ".", pkg)
		if err != nil {
			// Package may have no tests, skip it
			continue
		}

		lines := strings.Split(out, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Test") {
				allTests = append(allTests, testInfo{pkg: pkg, name: line})
			}
		}
	}

	return allTests, nil
}



