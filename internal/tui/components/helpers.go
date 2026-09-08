package components

import (
	"os"

	"golang.org/x/term"
)

// TerminalWidth is how wide the surface may draw, zero when there is nothing to
// measure. A wizard step builds its body before bubbletea has sized it, so a
// step whose body is a table has no other way to know what it has to fit in.
func TerminalWidth() int {
	cols, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || cols <= 0 {
		return 0
	}
	return cols
}

// PrintableWidth returns the visible character count of s, ignoring ANSI escape sequences.
func PrintableWidth(s string) int {
	inEscape := false
	n := 0
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}
