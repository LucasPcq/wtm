package components

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/rules"
)

type URLAtParams struct {
	// View is the frame as the program last drew it.
	View string
	X, Y int
}

// URLAt is the address under a click, read off the drawn frame rather than
// from a layout: a surface holding the mouse receives the click the terminal
// would otherwise have followed, and every address on screen is then
// clickable wherever it was drawn, without each one declaring where.
func URLAt(params URLAtParams) (string, bool) {
	lines := strings.Split(params.View, "\n")
	if params.Y < 0 || params.Y >= len(lines) {
		return "", false
	}
	plain := ansi.Strip(lines[params.Y])
	for _, span := range rules.URLSpans(plain) {
		start := ansi.StringWidth(plain[:span.Start])
		end := start + ansi.StringWidth(span.URL)
		if params.X >= start && params.X < end {
			return span.URL, true
		}
	}
	return "", false
}
