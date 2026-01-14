// Package exec provides command execution utilities.
package exec

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
)

// Sanitize replaces characters that are problematic in filenames with underscores.
func Sanitize(s string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		".", "_",
	)

	return replacer.Replace(s)
}

// Output runs a command and captures stdout only (stderr goes to os.Stderr).
// The trailing newline is trimmed from the output.
func Output(ctx context.Context, command string, args ...string) (string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	return strings.TrimSuffix(buf.String(), "\n"), err
}
