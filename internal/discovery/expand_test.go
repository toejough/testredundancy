package discovery_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/toejough/testredundancy/internal/discovery"
)

func TestExpandPackagesReturnsModuleRoot(t *testing.T) {
	pkgs, err := discovery.ExpandPackages("./...")
	if err != nil {
		t.Fatalf("ExpandPackages(./...) error = %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("ExpandPackages(./...) returned no packages")
	}

	modPath, err := modulePath()
	if err != nil {
		t.Fatalf("modulePath() error = %v", err)
	}

	if !contains(pkgs, modPath) {
		t.Fatalf("expected module root package %q in expanded packages", modPath)
	}
}

func TestExpandPackagesInvalidPattern(t *testing.T) {
	_, err := discovery.ExpandPackages("github.com/%%%")
	if err == nil {
		t.Fatal("expected error for invalid package pattern")
	}
}

func modulePath() (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Path}}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
