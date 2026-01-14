package exec_test

import (
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"github.com/toejough/testredundancy/internal/exec"
	"pgregory.net/rapid"
)

func TestSanitize_RemovesProblematicChars(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		expect := gomega.NewWithT(rt)

		input := rapid.String().Draw(rt, "input")
		result := exec.Sanitize(input)

		// Property: result should not contain any problematic filename characters
		expect.Expect(result).NotTo(gomega.ContainSubstring("/"))
		expect.Expect(result).NotTo(gomega.ContainSubstring("\\"))
		expect.Expect(result).NotTo(gomega.ContainSubstring(":"))
		expect.Expect(result).NotTo(gomega.ContainSubstring("."))
	})
}

func TestSanitize_PreservesSafeChars(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		expect := gomega.NewWithT(rt)

		// Generate string with only safe characters
		input := rapid.StringMatching(`[a-zA-Z0-9_-]*`).Draw(rt, "input")
		result := exec.Sanitize(input)

		// Property: safe characters should be unchanged
		expect.Expect(result).To(gomega.Equal(input))
	})
}

func TestSanitize_SameLength(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		expect := gomega.NewWithT(rt)

		input := rapid.String().Draw(rt, "input")
		result := exec.Sanitize(input)

		// Property: output length equals input length (1:1 character replacement)
		expect.Expect(len(result)).To(gomega.Equal(len(input)))
	})
}

func TestSanitize_ReplacesWithUnderscore(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// Specific example: path-like string
	input := "github.com/user/repo"
	result := exec.Sanitize(input)

	// Property: problematic chars become underscores
	expect.Expect(result).To(gomega.Equal("github_com_user_repo"))
	expect.Expect(strings.Count(result, "_")).To(gomega.Equal(3))
}
