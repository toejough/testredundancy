package analysis_test

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/toejough/testredundancy/internal/analysis"
)

func TestSelectMinimalSet_PreservesBaselineTests(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// Two tests that cover the same function
	coverage := []analysis.TestCoverage{
		{TestName: "pkg:TestBaseline", Coverage: map[string]float64{"funcA": 100}},
		{TestName: "pkg:TestOther", Coverage: map[string]float64{"funcA": 100}},
	}
	baseline := map[string]bool{"pkg:TestBaseline": true}
	targets := map[string]bool{"funcA": true}

	result := analysis.SelectMinimalSet(coverage, baseline, targets, 80.0)

	expect.Expect(result.KeptTests).To(gomega.ContainElement("pkg:TestBaseline"))
}

func TestSelectMinimalSet_RemovesTrulyRedundant(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// Two tests that cover the same function - only one needed
	coverage := []analysis.TestCoverage{
		{TestName: "pkg:TestA", Coverage: map[string]float64{"funcA": 100}},
		{TestName: "pkg:TestB", Coverage: map[string]float64{"funcA": 100}},
	}
	baseline := map[string]bool{}
	targets := map[string]bool{"funcA": true}

	result := analysis.SelectMinimalSet(coverage, baseline, targets, 80.0)

	// Should keep one and mark the other as redundant
	expect.Expect(result.KeptTests).To(gomega.HaveLen(1))
	expect.Expect(result.RedundantTests).To(gomega.HaveLen(1))
}

func TestSelectMinimalSet_KeepsTestsForUniqueContribution(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// Each test covers a unique function
	coverage := []analysis.TestCoverage{
		{TestName: "pkg:TestA", Coverage: map[string]float64{"funcA": 100, "funcB": 0}},
		{TestName: "pkg:TestB", Coverage: map[string]float64{"funcA": 0, "funcB": 100}},
	}
	baseline := map[string]bool{}
	targets := map[string]bool{"funcA": true, "funcB": true}

	result := analysis.SelectMinimalSet(coverage, baseline, targets, 80.0)

	// Both should be kept since each has unique contribution
	expect.Expect(result.KeptTests).To(gomega.HaveLen(2))
	expect.Expect(result.RedundantTests).To(gomega.BeEmpty())
}

func TestSelectMinimalSet_GreedySelectsBestFirst(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// TestBig covers more functions than TestSmall
	coverage := []analysis.TestCoverage{
		{TestName: "pkg:TestSmall", Coverage: map[string]float64{"funcA": 100, "funcB": 0, "funcC": 0}},
		{TestName: "pkg:TestBig", Coverage: map[string]float64{"funcA": 100, "funcB": 100, "funcC": 100}},
	}
	baseline := map[string]bool{}
	targets := map[string]bool{"funcA": true, "funcB": true, "funcC": true}

	result := analysis.SelectMinimalSet(coverage, baseline, targets, 80.0)

	// Should keep TestBig (covers all), TestSmall is redundant
	expect.Expect(result.KeptTests).To(gomega.ConsistOf("pkg:TestBig"))
	expect.Expect(result.RedundantTests).To(gomega.ConsistOf("pkg:TestSmall"))
}

func TestSelectMinimalSet_RespectsThreshold(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// TestPartial provides 50% coverage, threshold is 80%
	coverage := []analysis.TestCoverage{
		{TestName: "pkg:TestPartial", Coverage: map[string]float64{"funcA": 50}},
		{TestName: "pkg:TestFull", Coverage: map[string]float64{"funcA": 100}},
	}
	baseline := map[string]bool{}
	targets := map[string]bool{"funcA": true}

	result := analysis.SelectMinimalSet(coverage, baseline, targets, 80.0)

	// TestFull should be kept since TestPartial doesn't meet threshold
	expect.Expect(result.KeptTests).To(gomega.ContainElement("pkg:TestFull"))
}

func TestSelectMinimalSet_EmptyInputReturnsEmpty(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	result := analysis.SelectMinimalSet(nil, nil, nil, 80.0)

	expect.Expect(result.KeptTests).To(gomega.BeEmpty())
	expect.Expect(result.RedundantTests).To(gomega.BeEmpty())
}
