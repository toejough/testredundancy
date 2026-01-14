// Package exec provides command execution utilities.
package exec

import "strings"

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
