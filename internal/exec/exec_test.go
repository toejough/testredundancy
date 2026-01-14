package exec_test

import (
	"testing"

	"github.com/toejough/testredundancy/internal/exec"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
		{"with:colon", "with_colon"},
		{"with.dot", "with_dot"},
		{"github.com/foo/bar", "github_com_foo_bar"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := exec.Sanitize(tt.input)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
