package testredundancy_test

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/toejough/testredundancy"
)

func TestConfig_Defaults(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	config := testredundancy.Config{}

	// CoveragePackages should default to empty (will be set to ./... at runtime)
	expect.Expect(config.CoveragePackages).To(gomega.BeEmpty())
	expect.Expect(config.CoverageThreshold).To(gomega.BeZero())
}

func TestBaselineTestSpec_Fields(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	spec := testredundancy.BaselineTestSpec{
		Package:     "./uat/...",
		TestPattern: "TestGolden.*",
	}

	expect.Expect(spec.Package).To(gomega.Equal("./uat/..."))
	expect.Expect(spec.TestPattern).To(gomega.Equal("TestGolden.*"))
}

func TestResult_Fields(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	result := testredundancy.Result{
		KeptTests:      []string{"pkg:TestA"},
		RedundantTests: []string{"pkg:TestB"},
	}

	expect.Expect(result.KeptTests).To(gomega.ConsistOf("pkg:TestA"))
	expect.Expect(result.RedundantTests).To(gomega.ConsistOf("pkg:TestB"))
}
