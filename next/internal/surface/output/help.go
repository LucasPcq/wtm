package output

import (
	"io"
	"strings"
)

func Help(w io.Writer, text string) {
	blank(w)
	for line := range strings.Lines(text) {
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			blank(w)
			continue
		}
		message(w, line)
	}
	blank(w)
}
