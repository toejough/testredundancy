package coverage_test

import (
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"github.com/toejough/testredundancy/internal/coverage"
	"pgregory.net/rapid"
)

func TestParseBlockID_ValidFormat(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	file, startLine, startCol, endLine, endCol, err := coverage.ParseBlockID(
		"github.com/user/repo/file.go:10.5,20.15")

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(file).To(gomega.Equal("github.com/user/repo/file.go"))
	expect.Expect(startLine).To(gomega.Equal(10))
	expect.Expect(startCol).To(gomega.Equal(5))
	expect.Expect(endLine).To(gomega.Equal(20))
	expect.Expect(endCol).To(gomega.Equal(15))
}

func TestParseBlockID_InvalidFormat(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// Missing colon
	_, _, _, _, _, err := coverage.ParseBlockID("file.go10.5,20.15")
	expect.Expect(err).To(gomega.HaveOccurred())

	// Missing comma
	_, _, _, _, _, err = coverage.ParseBlockID("file.go:10.520.15")
	expect.Expect(err).To(gomega.HaveOccurred())

	// Missing dot in start position
	_, _, _, _, _, err = coverage.ParseBlockID("file.go:105,20.15")
	expect.Expect(err).To(gomega.HaveOccurred())
}

func TestParseBlock_ValidLine(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	block, err := coverage.ParseBlock("github.com/repo/file.go:10.5,20.15 3 1")

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(block.File).To(gomega.Equal("github.com/repo/file.go"))
	expect.Expect(block.StartLine).To(gomega.Equal(10))
	expect.Expect(block.StartCol).To(gomega.Equal(5))
	expect.Expect(block.EndLine).To(gomega.Equal(20))
	expect.Expect(block.EndCol).To(gomega.Equal(15))
	expect.Expect(block.Statements).To(gomega.Equal(3))
	expect.Expect(block.Count).To(gomega.Equal(1))
}

func TestParseBlock_InvalidLine(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// Missing fields
	_, err := coverage.ParseBlock("file.go:10.5,20.15 3")
	expect.Expect(err).To(gomega.HaveOccurred())

	// Too many fields
	_, err = coverage.ParseBlock("file.go:10.5,20.15 3 1 extra")
	expect.Expect(err).To(gomega.HaveOccurred())
}

func TestParseBlock_RoundTrip(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		expect := gomega.NewWithT(rt)

		// Generate valid block components
		file := rapid.StringMatching(`[a-zA-Z0-9/_.-]+\.go`).Draw(rt, "file")
		startLine := rapid.IntRange(1, 10000).Draw(rt, "startLine")
		startCol := rapid.IntRange(1, 200).Draw(rt, "startCol")
		endLine := rapid.IntRange(startLine, startLine+100).Draw(rt, "endLine")
		endCol := rapid.IntRange(1, 200).Draw(rt, "endCol")
		statements := rapid.IntRange(0, 100).Draw(rt, "statements")
		count := rapid.IntRange(0, 1000).Draw(rt, "count")

		// Format as coverage line
		line := coverage.FormatBlock(coverage.Block{
			File:       file,
			StartLine:  startLine,
			StartCol:   startCol,
			EndLine:    endLine,
			EndCol:     endCol,
			Statements: statements,
			Count:      count,
		})

		// Parse should recover original values
		block, err := coverage.ParseBlock(line)
		expect.Expect(err).NotTo(gomega.HaveOccurred())
		expect.Expect(block.File).To(gomega.Equal(file))
		expect.Expect(block.StartLine).To(gomega.Equal(startLine))
		expect.Expect(block.StartCol).To(gomega.Equal(startCol))
		expect.Expect(block.EndLine).To(gomega.Equal(endLine))
		expect.Expect(block.EndCol).To(gomega.Equal(endCol))
		expect.Expect(block.Statements).To(gomega.Equal(statements))
		expect.Expect(block.Count).To(gomega.Equal(count))
	})
}

func TestMergeBlocks_SumsDuplicateCounts(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	input := `mode: set
file.go:10.5,20.15 3 1
file.go:10.5,20.15 3 2
file.go:30.1,40.10 5 1
`
	result, err := coverage.MergeBlocks(input)

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	// Should have mode line + 2 unique blocks
	lines := strings.Split(strings.TrimSpace(result), "\n")
	expect.Expect(lines).To(gomega.HaveLen(3))
	expect.Expect(lines[0]).To(gomega.Equal("mode: set"))

	// Find the merged block - count should be 3 (1+2)
	var foundMerged bool
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "file.go:10.5,20.15") {
			expect.Expect(line).To(gomega.HaveSuffix(" 3")) // count = 3
			foundMerged = true
		}
	}
	expect.Expect(foundMerged).To(gomega.BeTrue())
}

func TestMergeBlocks_PreservesModeLine(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	input := `mode: atomic
file.go:10.5,20.15 3 1
`
	result, err := coverage.MergeBlocks(input)

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(result).To(gomega.HavePrefix("mode: atomic\n"))
}

func TestMergeBlocks_HandlesEmptyInput(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	result, err := coverage.MergeBlocks("")

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(result).To(gomega.BeEmpty())
}

func TestFilterQtpl_RemovesQtplEntries(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	input := `mode: set
file.go:10.5,20.15 3 1
template.qtpl:5.1,10.5 2 1
other.go:30.1,40.10 5 1
`
	result, err := coverage.FilterQtpl(input)

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	lines := strings.Split(strings.TrimSpace(result), "\n")
	expect.Expect(lines).To(gomega.HaveLen(3)) // mode + 2 non-qtpl files
	expect.Expect(result).NotTo(gomega.ContainSubstring(".qtpl"))
}

func TestFilterQtpl_PreservesModeLine(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	input := `mode: atomic
file.go:10.5,20.15 3 1
`
	result, err := coverage.FilterQtpl(input)

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(result).To(gomega.HavePrefix("mode: atomic\n"))
}

func TestFilterQtpl_HandlesEmptyInput(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	_, err := coverage.FilterQtpl("")

	expect.Expect(err).To(gomega.HaveOccurred())
}

func TestParseFunctionCoverage_ValidOutput(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	// Sample output from go tool cover -func
	input := `github.com/user/repo/file.go:10:	FuncA		85.7%
github.com/user/repo/file.go:25:	FuncB		100.0%
github.com/user/repo/other.go:5:	FuncC		0.0%
total:							(statements)	62.5%
`
	funcs, err := coverage.ParseFunctionCoverage(input)

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(funcs).To(gomega.HaveLen(3))
	expect.Expect(funcs["github.com/user/repo/file.go:10: FuncA"]).To(gomega.BeNumerically("~", 85.7, 0.01))
	expect.Expect(funcs["github.com/user/repo/file.go:25: FuncB"]).To(gomega.BeNumerically("~", 100.0, 0.01))
	expect.Expect(funcs["github.com/user/repo/other.go:5: FuncC"]).To(gomega.BeNumerically("~", 0.0, 0.01))
}

func TestParseFunctionCoverage_SkipsTotalLine(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	input := `github.com/user/repo/file.go:10:	FuncA		85.7%
total:							(statements)	62.5%
`
	funcs, err := coverage.ParseFunctionCoverage(input)

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(funcs).To(gomega.HaveLen(1))
}

func TestParseFunctionCoverage_HandlesEmptyInput(t *testing.T) {
	t.Parallel()
	expect := gomega.NewWithT(t)

	funcs, err := coverage.ParseFunctionCoverage("")

	expect.Expect(err).NotTo(gomega.HaveOccurred())
	expect.Expect(funcs).To(gomega.BeEmpty())
}
