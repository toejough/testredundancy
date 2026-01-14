// Package exec provides command execution utilities.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Output runs a command and captures stdout only (stderr goes to os.Stderr).
func Output(ctx context.Context, command string, args ...string) (string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	return strings.TrimSuffix(buf.String(), "\n"), err
}

// RunQuietCoverage runs a command and filters out expected coverage warnings.
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
		fmt.Fprintln(os.Stderr, line)
	}

	return err
}

// Sanitize makes a string safe for use in filenames.
func Sanitize(s string) string {
	// Replace characters that are problematic in filenames
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		".", "_",
	)

	return replacer.Replace(s)
}
