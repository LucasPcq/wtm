package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const (
	indent         = "  "
	dot            = " · "
	glyphSuccess   = "✓"
	glyphAttention = "!"
	glyphFailure   = "✗"
	glyphUnchanged = "="
	glyphProgress  = "›"
	glyphNext      = "→"
	noteSeparator  = "   "
	ellipsis       = "…"
	tabWidth       = 8
)

func successLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", indent, success.Render(glyphSuccess), msg)
}

func warningLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", indent, warning.Render(glyphAttention), msg)
}

func unchangedLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", indent, muted.Render(glyphUnchanged), muted.Render(msg))
}

func errorLine(w io.Writer, msg string) {
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	fmt.Fprintf(w, "%s%s %s\n", indent, danger.Render(glyphFailure), lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%s  %s\n", indent, line)
	}
}

func loadingLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", indent, muted.Render(glyphProgress), muted.Render(msg))
}

func infoLine(w io.Writer, label, value string) {
	fmt.Fprintf(w, "%s%s  %s\n", indent, muted.Render(label), value)
}

func sectionTitle(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s\n", indent, bold.Render(title))
}

func message(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s\n", indent, msg)
}

func blank(w io.Writer) {
	fmt.Fprintln(w)
}

type NextStep struct {
	Command string
	Note    string
}

func nextSteps(w io.Writer, steps []NextStep) {
	width := 0
	for _, step := range steps {
		width = max(width, len(step.Command))
	}
	for _, step := range steps {
		line := bold.Render(step.Command)
		if step.Note != "" {
			line = bold.Render(fmt.Sprintf("%-*s", width, step.Command)) + muted.Render(noteSeparator+step.Note)
		}
		fmt.Fprintf(w, "%s%s %s\n", indent, primary.Render(glyphNext), line)
	}
}

func blockWidth(w io.Writer) int {
	cols := terminalWidth(w)
	if cols <= 0 {
		return hookFallbackWidth
	}
	return max(cols-2*len(indent), hookMinWidth)
}

func truncate(value string, width int) string {
	return ansi.Truncate(value, width, ellipsis)
}

func expandTabs(line string) string {
	return strings.ReplaceAll(line, "\t", strings.Repeat(" ", tabWidth))
}

func terminalWidth(w io.Writer) int {
	stream, bars := unwrapStream(w)
	file, ok := stream.(*os.File)
	if !ok {
		return 0
	}
	cols, _, err := term.GetSize(int(file.Fd()))
	if err != nil || cols <= 0 {
		return 0
	}
	return max(cols-bars*accentBarWidth, 0)
}

func IsTerminal(w io.Writer) bool {
	stream, _ := unwrapStream(w)
	file, ok := stream.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func unwrapStream(w io.Writer) (io.Writer, int) {
	bars := 0
	for {
		unwrapper, ok := w.(interface{ Unwrap() io.Writer })
		if !ok {
			return w, bars
		}
		w, bars = unwrapper.Unwrap(), bars+1
	}
}
