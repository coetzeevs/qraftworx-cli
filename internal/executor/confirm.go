package executor

import (
	"regexp"
	"strings"
	"unicode"
)

// ansiPattern matches ANSI escape sequences (CSI, OSC, and single-char escapes).
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[^[\]()]`)

// stripControlChars removes all ANSI escape sequences and non-printable
// characters from the input. Used to sanitize tool args before display (S1).
func stripControlChars(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) || r == '\n' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
