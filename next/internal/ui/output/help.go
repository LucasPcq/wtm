package output

import (
	"io"
	"strings"
)

// Help writes a command's help text to w, each line indented like the rest of wtm's output and set off by a blank line above and below.
func Help(w io.Writer, text string) {
	Blank(w)
	for line := range strings.Lines(text) {
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			Blank(w)
			continue
		}
		Message(w, line)
	}
	Blank(w)
}
