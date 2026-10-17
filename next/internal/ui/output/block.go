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
	Indent         = "  "
	Dot            = " · "
	glyphSuccess   = "✓"
	GlyphAttention = "!"
	GlyphFailure   = "✗"
	glyphUnchanged = "="
	glyphProgress  = "›"
	glyphNext      = "→"
	noteSeparator  = "   "
	ellipsis       = "…"
	tabWidth       = 8
)

func SuccessLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, success.Render(glyphSuccess), msg)
}

func WarningLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, Warning.Render(GlyphAttention), msg)
}

func UnchangedLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, Muted.Render(glyphUnchanged), Muted.Render(msg))
}

func ErrorLine(w io.Writer, msg string) {
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	fmt.Fprintf(w, "%s%s %s\n", Indent, Danger.Render(GlyphFailure), lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%s  %s\n", Indent, line)
	}
}

func loadingLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, Muted.Render(glyphProgress), Muted.Render(msg))
}

type Info struct {
	Label string
	Value string
}

func InfoLine(w io.Writer, line Info) {
	fmt.Fprintf(w, "%s%s  %s\n", Indent, Muted.Render(line.Label), line.Value)
}

func SectionTitle(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s\n", Indent, bold.Render(title))
}

func Message(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s\n", Indent, msg)
}

func Blank(w io.Writer) {
	fmt.Fprintln(w)
}

type NextStep struct {
	Command string
	Note    string
}

func NextSteps(w io.Writer, steps []NextStep) {
	width := 0
	for _, step := range steps {
		width = max(width, len(step.Command))
	}
	for _, step := range steps {
		line := bold.Render(step.Command)
		if step.Note != "" {
			line = bold.Render(fmt.Sprintf("%-*s", width, step.Command)) + Muted.Render(noteSeparator+step.Note)
		}
		fmt.Fprintf(w, "%s%s %s\n", Indent, primary.Render(glyphNext), line)
	}
}

func blockWidth(w io.Writer) int {
	cols := terminalWidth(w)
	if cols <= 0 {
		return hookFallbackWidth
	}
	return max(cols-2*len(Indent), hookMinWidth)
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
