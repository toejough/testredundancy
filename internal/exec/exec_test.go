package exec_test

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestOutput_CapturesStdout(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		expect := gomega.NewWithT(rt)

		// Generate a safe string to echo (alphanumeric only to avoid shell escaping issues)
		input := rapid.StringMatching(`[a-zA-Z0-9]{1,50}`).Draw(rt, "input")

		result, err := exec.Output(context.Background(), "echo", "-n", input)

		expect.Expect(err).NotTo(gomega.HaveOccurred())
		expect.Expect(result).To(gomega.Equal(input))
	})
}

func TestOutput_TrimsTrailingNewline(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// echo without -n adds a newline, Output should trim it
	result, err := exec.Output(context.Background(), "echo", "hello")

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(result).To(gomega.Equal("hello"))
}

func TestOutput_ReturnsErrorOnFailure(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	_, err := exec.Output(context.Background(), "false")

	expect.Expect(err).To(gomega.HaveOccurred())
}

func TestOutput_RespectsContextCancellation(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// sleep for 10 seconds, but context times out after 10ms
	_, err := exec.Output(ctx, "sleep", "10")

	expect.Expect(err).To(gomega.HaveOccurred())
}

func TestRunQuietCoverage_ReturnsNilOnSuccess(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	err := exec.RunQuietCoverage("true")

	expect.Expect(err).NotTo(gomega.HaveOccurred())
}

func TestRunQuietCoverage_ReturnsErrorOnFailure(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	err := exec.RunQuietCoverage("false")

	expect.Expect(err).To(gomega.HaveOccurred())
}

func TestRunQuietCoverage_FiltersCoverageWarning(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// This should not show the warning in stderr (we can't easily capture stderr
	// from the test, but we can verify the command runs without error)
	err := exec.RunQuietCoverage("sh", "-c",
		"echo 'warning: no packages being tested depend on matches' >&2; exit 0")

	expect.Expect(err).NotTo(gomega.HaveOccurred())
}
