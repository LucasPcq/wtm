package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// The marks of wtm's human output: Indent opens every line of a block, Dot joins the parts of one line, and each glyph says what its line reports.
const (
	Indent         = "  "
	Dot            = " · "
	GlyphSuccess   = "✓"
	GlyphAttention = "!"
	GlyphFailure   = "✗"
	glyphUnchanged = "="
	glyphProgress  = "›"
	glyphNext      = "→"
	noteSeparator  = "   "
	ellipsis       = "…"
	tabWidth       = 8
)

// SuccessLine writes msg to w as something done.
func SuccessLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, StylesOf(w).Success.Render(GlyphSuccess), msg)
}

// WarningLine writes msg to w as something the reader should look at.
func WarningLine(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, StylesOf(w).Warning.Render(GlyphAttention), msg)
}

// UnchangedLine writes msg to w, muted, as something left as it was.
func UnchangedLine(w io.Writer, msg string) {
	muted := StylesOf(w).Muted
	fmt.Fprintf(w, "%s%s %s\n", Indent, muted.Render(glyphUnchanged), muted.Render(msg))
}

// ErrorLine writes msg to w as a failure, its first line after the glyph and the others hung under it.
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

// An Info is a fact a block shows as a muted label and its value.
type Info struct {
	Label string
	Value string
}

// InfoLine writes line to w.
func InfoLine(w io.Writer, line Info) {
	fmt.Fprintf(w, "%s%s  %s\n", Indent, StylesOf(w).Muted.Render(line.Label), line.Value)
}

// SectionTitle writes title to w in bold, above the lines of its section.
func SectionTitle(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s\n", Indent, StylesOf(w).bold.Render(title))
}

// Message writes msg to w as a plain indented line.
func Message(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s\n", Indent, msg)
}

// Blank writes an empty line to w.
func Blank(w io.Writer) {
	fmt.Fprintln(w)
}

// A NextStep is a command the reader may run next, with an optional note on what it does.
type NextStep struct {
	Command string
	Note    string
}

// NextSteps writes steps to w, one per line, their notes aligned in one column.
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

// blockWidth returns the columns a line of a block may fill on w, or a fallback width when the terminal's is unknown.
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
