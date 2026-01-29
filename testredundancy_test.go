package testredundancy_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toejough/testredundancy"
)

func TestBaselinePatternExpandsAcrossPackages(t *testing.T) {
	root := writeTempModule(t)

	config := testredundancy.Config{
		BaselineTests: []testredundancy.BaselineTestSpec{
			{Package: "./...", TestPattern: "TestProperty_"},
		},
		CoverageThreshold: 0,
		PackageToAnalyze:  "./...",
		CoveragePackages:  "./...",
	}

	output, err := runFindInModule(t, root, config)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}

	if !strings.Contains(output, "Found 2 baseline tests") {
		t.Fatalf("expected output to include baseline count 2, got: %s", output)
	}
}

func TestBaselineEmptyPatternNoMatches(t *testing.T) {
	root := writeTempModule(t)

	config := testredundancy.Config{
		BaselineTests: []testredundancy.BaselineTestSpec{
			{Package: "./...", TestPattern: ""},
		},
		CoverageThreshold: 0,
		PackageToAnalyze:  "./...",
		CoveragePackages:  "./...",
	}

	output, err := runFindInModule(t, root, config)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}

	if !strings.Contains(output, "Found 0 baseline tests") {
		t.Fatalf("expected output to include baseline count 0, got: %s", output)
	}
}

func TestBaselineNoPackagesNoError(t *testing.T) {
	root := writeTempModule(t)

	config := testredundancy.Config{
		BaselineTests: []testredundancy.BaselineTestSpec{
			{Package: "./.../doesnotexist", TestPattern: "TestProperty_"},
		},
		CoverageThreshold: 0,
		PackageToAnalyze:  "./...",
		CoveragePackages:  "./...",
	}

	output, err := runFindInModule(t, root, config)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}

	if !strings.Contains(output, "Found 0 baseline tests") {
		t.Fatalf("expected output to include baseline count 0, got: %s", output)
	}
}

func TestBaselineExpansionErrorSurfaces(t *testing.T) {
	root := writeTempModule(t)

	config := testredundancy.Config{
		BaselineTests: []testredundancy.BaselineTestSpec{
			{Package: "github.com/%%%", TestPattern: "TestProperty_"},
		},
		CoverageThreshold: 0,
		PackageToAnalyze:  "./...",
		CoveragePackages:  "./...",
	}

	_, err := runFindInModule(t, root, config)
	if err == nil {
		t.Fatal("expected error for invalid package pattern")
	}
}

func writeTempModule(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/tmpmod\n\ngo 1.20\n")

	writePackage(t, root, "pkgone", "TestProperty_A", "TestOther")
	writePackage(t, root, "pkgtwo", "TestProperty_B")

	return root
}

func writePackage(t *testing.T, root, name string, tests ...string) {
	t.Helper()

	pkgDir := filepath.Join(root, name)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pkgDir, err)
	}

	writeFile(t, filepath.Join(pkgDir, "foo.go"), "package "+name+"\n\nfunc Foo() int { return 1 }\n")

	var b strings.Builder
	b.WriteString("package " + name + "\n\nimport \"testing\"\n\n")
	for _, testName := range tests {
		b.WriteString("func " + testName + "(t *testing.T) {}\n")
	}

	writeFile(t, filepath.Join(pkgDir, "foo_test.go"), b.String())
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func captureOutput(fn func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	callErr := fn()

	_ = w.Close()
	os.Stdout = oldStdout

	out, readErr := io.ReadAll(r)
	if readErr != nil {
		return "", readErr
	}

	return string(out), callErr
}

func runFindInModule(t *testing.T, root string, config testredundancy.Config) (string, error) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.Chdir(root); err != nil {
		return "", err
	}

	return captureOutput(func() error {
		return testredundancy.Find(config)
	})
}
