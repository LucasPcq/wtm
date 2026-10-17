package output

import (
	"io"
	"strings"
)

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
