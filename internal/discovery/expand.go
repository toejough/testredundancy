package discovery

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	executil "github.com/toejough/testredundancy/internal/exec"
)

// ExpandPackages expands a Go package pattern (e.g., ./...) relative to the module root.
func ExpandPackages(pattern string) ([]string, error) {
	modRoot, err := moduleRoot()
	if err != nil {
		return nil, err
	}

	out, err := executil.OutputInDir(context.Background(), modRoot, "go", "list", pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	packages := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		packages = append(packages, line)
	}

	return packages, nil
}

func moduleRoot() (string, error) {
	gomod, err := executil.Output(context.Background(), "go", "env", "GOMOD")
	if err != nil {
		return "", fmt.Errorf("failed to resolve module root: %w", err)
	}

	gomod = strings.TrimSpace(gomod)
	if gomod == "" || gomod == "NUL" {
		return "", fmt.Errorf("failed to resolve module root: go env GOMOD returned empty")
	}

	return filepath.Dir(gomod), nil
}
