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

// RunQuietCoverage runs a command, filtering out expected warnings about packages
// not matching coverage patterns from stderr.
func RunQuietCoverage(command string, arg ...string) error {
	cmd := exec.Command(command, arg...)
	cmd.Stdin = os.Stdin

	// Capture stderr to filter out coverage warnings
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Filter and display stderr, removing expected coverage warnings
	stderrLines := strings.Split(stderrBuf.String(), "\n")

	for _, line := range stderrLines {
		// Skip the "no packages being tested depend on matches" warning
		if strings.Contains(line, "no packages being tested depend on matches") {
			continue
		}
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Show other stderr output
		os.Stderr.WriteString(line + "\n")
	}

	return err
}
