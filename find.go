package testredundancy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/toejough/testredundancy/internal/analysis"
	"github.com/toejough/testredundancy/internal/coverage"
	"github.com/toejough/testredundancy/internal/discovery"
	"github.com/toejough/testredundancy/internal/exec"
)

// Find identifies redundant tests based on the config.
// It runs each test individually to collect coverage, then uses a greedy
// algorithm to find the minimal set of tests that maintain coverage.
func Find(ctx context.Context, config Config) (*Result, error) {
	fmt.Println("Finding redundant tests...")
	fmt.Println()

	// Default to ./... if not specified
	coverpkg := config.CoveragePackages
	if coverpkg == "" {
		coverpkg = "./..."
	}

	// Step 1: Identify baseline tests
	fmt.Println("Step 1: Identifying baseline tests...")
	baselineTestSet := make(map[string]bool)

	for _, spec := range config.BaselineTests {
		if spec.TestPattern != "" {
			fullPkg, err := exec.Output(ctx, "go", "list", spec.Package)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve package %s: %w", spec.Package, err)
			}

			baselineTestSet[strings.TrimSpace(fullPkg)+":"+spec.TestPattern] = true
		} else {
			pkgTests, err := listTests(ctx, spec.Package)
			if err != nil {
				fmt.Printf("  Warning: couldn't list tests in %s: %v\n", spec.Package, err)
			} else {
				for _, t := range pkgTests {
					baselineTestSet[t.QualifiedName()] = true
				}
			}
		}
	}

	fmt.Printf("  Identified %d baseline tests\n", len(baselineTestSet))

	// Step 2: List all tests
	fmt.Println("\nStep 2: Listing all tests...")

	allTests, err := listTests(ctx, config.PackageToAnalyze)
	if err != nil {
		return nil, fmt.Errorf("failed to list tests: %w", err)
	}

	var baselineTests []discovery.TestInfo
	var nonBaselineTests []discovery.TestInfo

	for _, t := range allTests {
		if baselineTestSet[t.QualifiedName()] {
			baselineTests = append(baselineTests, t)
		} else {
			nonBaselineTests = append(nonBaselineTests, t)
		}
	}

	fmt.Printf("  Found %d baseline tests, %d non-baseline tests (%d total)\n",
		len(baselineTests), len(nonBaselineTests), len(allTests))

	// Step 3: Run each test individually to collect coverage
	fmt.Println("\nStep 3: Running each test individually to collect coverage...")

	allTestsToRun := append(baselineTests, nonBaselineTests...)

	testCoverageFiles := make(map[string]string)
	var testCoverageFilesMu sync.Mutex

	// Run tests concurrently
	fmt.Printf("  Running %d tests...\n", len(allTestsToRun))

	numWorkers := runtime.NumCPU()
	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup
	var completed int32

	for _, test := range allTestsToRun {
		wg.Add(1)

		go func(test discovery.TestInfo) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			coverFile := fmt.Sprintf("cov_%s_%s.out", exec.Sanitize(filepath.Base(test.Pkg)), test.Name)
			coverFileRaw := coverFile + ".raw"

			testErr := exec.RunQuietCoverage("go", "test", "-count=1", "-coverprofile="+coverFileRaw, "-coverpkg="+coverpkg,
				"-run", "^"+test.Name+"$", test.Pkg)

			current := atomic.AddInt32(&completed, 1)

			if testErr != nil {
				fmt.Printf("    [%d/%d] %s... FAILED\n", current, len(allTestsToRun), test.QualifiedName())

				return
			}

			// Filter qtpl files
			data, readErr := os.ReadFile(coverFileRaw)
			if readErr != nil {
				fmt.Printf("    [%d/%d] %s... FAILED (read)\n", current, len(allTestsToRun), test.QualifiedName())
				os.Remove(coverFileRaw)

				return
			}

			filtered, filterErr := coverage.FilterQtpl(string(data))
			if filterErr != nil {
				fmt.Printf("    [%d/%d] %s... FAILED (filter)\n", current, len(allTestsToRun), test.QualifiedName())
				os.Remove(coverFileRaw)

				return
			}

			if writeErr := os.WriteFile(coverFile, []byte(filtered), 0o600); writeErr != nil {
				fmt.Printf("    [%d/%d] %s... FAILED (write)\n", current, len(allTestsToRun), test.QualifiedName())
				os.Remove(coverFileRaw)

				return
			}

			os.Remove(coverFileRaw)

			testCoverageFilesMu.Lock()
			testCoverageFiles[test.QualifiedName()] = coverFile
			testCoverageFilesMu.Unlock()

			fmt.Printf("    [%d/%d] %s... OK\n", current, len(allTestsToRun), test.QualifiedName())
		}(test)
	}

	wg.Wait()

	// Step 4: Compute target coverage
	fmt.Println("\nStep 4: Computing target coverage with all tests...")

	var allCoverageFiles []string
	for _, f := range testCoverageFiles {
		allCoverageFiles = append(allCoverageFiles, f)
	}

	if len(allCoverageFiles) == 0 {
		return nil, fmt.Errorf("no tests ran successfully")
	}

	// Read all coverage files
	var coverageContents []string
	for _, f := range allCoverageFiles {
		data, readErr := os.ReadFile(f)
		if readErr != nil {
			continue
		}

		coverageContents = append(coverageContents, string(data))
	}

	// Merge all coverage
	mergedCoverage, err := coverage.MergeContents(coverageContents)
	if err != nil {
		return nil, fmt.Errorf("failed to merge total coverage: %w", err)
	}

	// Write merged coverage to temp file for go tool cover
	totalCoverageFile := "total_coverage.out"

	if err := os.WriteFile(totalCoverageFile, []byte(mergedCoverage), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write merged coverage: %w", err)
	}

	defer os.Remove(totalCoverageFile)

	// Get function coverage
	coverOutput, err := exec.Output(ctx, "go", "tool", "cover", "-func="+totalCoverageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze coverage: %w", err)
	}

	targetCoverage, err := coverage.ParseFunctionCoverage(coverOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse coverage: %w", err)
	}

	// Build target set: functions at threshold
	targetFuncs := make(map[string]bool)

	for fn, cov := range targetCoverage {
		if cov >= config.CoverageThreshold {
			targetFuncs[fn] = true
		}
	}

	fmt.Printf("  Target: %d functions at %.0f%%+ (with all tests)\n", len(targetFuncs), config.CoverageThreshold)

	// Step 5: Build coverage data for each test
	fmt.Println("\nStep 5: Building minimal test set...")

	var testCoverages []analysis.TestCoverage

	for _, test := range allTestsToRun {
		coverFile, ok := testCoverageFiles[test.QualifiedName()]
		if !ok {
			continue
		}

		data, readErr := os.ReadFile(coverFile)
		if readErr != nil {
			continue
		}

		// Write to temp file for go tool cover
		tempFile := "temp_" + filepath.Base(coverFile)

		if writeErr := os.WriteFile(tempFile, data, 0o600); writeErr != nil {
			continue
		}

		coverOutput, coverErr := exec.Output(ctx, "go", "tool", "cover", "-func="+tempFile)
		os.Remove(tempFile)

		if coverErr != nil {
			continue
		}

		funcCov, parseErr := coverage.ParseFunctionCoverage(coverOutput)
		if parseErr != nil {
			continue
		}

		testCoverages = append(testCoverages, analysis.TestCoverage{
			TestName: test.QualifiedName(),
			Coverage: funcCov,
		})
	}

	// Run greedy selection
	result := analysis.SelectMinimalSet(testCoverages, baselineTestSet, targetFuncs, config.CoverageThreshold)

	// Print results
	fmt.Printf("\nResults:\n")
	fmt.Printf("  Kept tests: %d\n", len(result.KeptTests))
	fmt.Printf("  Redundant tests: %d\n", len(result.RedundantTests))

	if len(result.RedundantTests) > 0 {
		fmt.Println("\nRedundant tests (can be removed):")

		for _, t := range result.RedundantTests {
			fmt.Printf("  - %s\n", t)
		}
	}

	// Cleanup coverage files
	for _, f := range testCoverageFiles {
		os.Remove(f)
	}

	return &Result{
		KeptTests:      result.KeptTests,
		RedundantTests: result.RedundantTests,
	}, nil
}

// listTests lists all test functions in a package pattern.
func listTests(ctx context.Context, pkgPattern string) ([]discovery.TestInfo, error) {
	listOut, err := exec.Output(ctx, "go", "list", pkgPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	var allTests []discovery.TestInfo
	packages := strings.Split(strings.TrimSpace(listOut), "\n")

	for _, pkg := range packages {
		if pkg == "" {
			continue
		}

		out, testErr := exec.Output(ctx, "go", "test", "-list", ".", pkg)
		if testErr != nil {
			continue
		}

		tests := discovery.ParseTestOutput(pkg, out)
		allTests = append(allTests, tests...)
	}

	return allTests, nil
}
