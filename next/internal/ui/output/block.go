package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
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
	fmt.Fprintf(w, "%s%s %s\n", Indent, StylesOf(w).success.Render(glyphSuccess), msg)
}

func WarningLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, StylesOf(w).Warning.Render(GlyphAttention), msg)
}

func UnchangedLine(w io.Writer, msg string) {
	muted := StylesOf(w).Muted
	fmt.Fprintf(w, "%s%s %s\n", Indent, muted.Render(glyphUnchanged), muted.Render(msg))
}

func ErrorLine(w io.Writer, msg string) {
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	fmt.Fprintf(w, "%s%s %s\n", Indent, StylesOf(w).Danger.Render(GlyphFailure), lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%s  %s\n", Indent, line)
	}
}

func loadingLine(w io.Writer, msg string) {
	muted := StylesOf(w).Muted
	fmt.Fprintf(w, "%s%s %s\n", Indent, muted.Render(glyphProgress), muted.Render(msg))
}

type Info struct {
	Label string
	Value string
}

func InfoLine(w io.Writer, line Info) {
	fmt.Fprintf(w, "%s%s  %s\n", Indent, StylesOf(w).Muted.Render(line.Label), line.Value)
}

func SectionTitle(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s\n", Indent, StylesOf(w).bold.Render(title))
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
	st := StylesOf(w)
	width := 0
	for _, step := range steps {
		width = max(width, len(step.Command))
	}
	for _, step := range steps {
		line := st.bold.Render(step.Command)
		if step.Note != "" {
			line = st.bold.Render(fmt.Sprintf("%-*s", width, step.Command)) + st.Muted.Render(noteSeparator+step.Note)
		}
		fmt.Fprintf(w, "%s%s %s\n", Indent, st.primary.Render(glyphNext), line)
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
