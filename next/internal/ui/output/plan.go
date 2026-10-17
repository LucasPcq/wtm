package output

import (
	"cmp"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	planGap          = "  "
	planContinuation = "  "
	planSeparator    = ", "
)

// NothingToDo is what a plan with no row shows.
const NothingToDo = "nothing to do"

// A PlanRow is one line of a plan: a label and the values it applies to.
type PlanRow struct {
	Label  string
	Values []string
}

// PlanParams is a plan to draw: its rows, the warnings under them, and the width and styles to draw them with.
type PlanParams struct {
	Rows     []PlanRow
	Warnings []string
	Width    int     // the columns a line may fill; 0 or less wraps nothing
	Styles   *Styles // nil draws no colour
}

// PlanLines returns the lines of the plan params describes: its rows with their values aligned in one column and wrapped to its width, then its warnings after an empty line.
func PlanLines(params PlanParams) []string {
	st := cmp.Or(params.Styles, plainStyles)
	lines := planRows(params, st)

	if len(params.Warnings) > 0 {
		lines = append(lines, "")
	}
	for _, text := range params.Warnings {
		first, rest, _ := strings.Cut(text, "\n")
		lines = append(lines, st.Warning.Render(GlyphAttention)+" "+first)
		for line := range strings.SplitSeq(rest, "\n") {
			if line != "" {
				lines = append(lines, planContinuation+line)
			}
		}
	}
	return lines
}

func planRows(params PlanParams, st *Styles) []string {
	if len(params.Rows) == 0 {
		return []string{st.Muted.Render(glyphUnchanged + " " + NothingToDo)}
	}
	labels := 0
	for _, row := range params.Rows {
		labels = max(labels, lipgloss.Width(row.Label))
	}
	column := labels + len(planGap)

	var lines []string
	for _, row := range params.Rows {
		values := planValues(row.Values, params.Width-column)
		lines = append(lines, st.Muted.Render(row.Label)+strings.Repeat(" ", column-lipgloss.Width(row.Label))+values[0])
		for _, value := range values[1:] {
			lines = append(lines, strings.Repeat(" ", column)+value)
		}
	}
	return lines
}

// planValues returns values on one line when they fit width, else one value per line, each wrapped to width.
func planValues(values []string, width int) []string {
	joined := strings.Join(values, planSeparator)
	if len(values) < 2 || width < 1 || lipgloss.Width(joined) <= width {
		return wrapped(joined, width)
	}

	var lines []string
	for _, value := range values {
		lines = append(lines, wrapped(value, width)...)
	}
	return lines
}

// wrapped returns value wrapped at its spaces to width, a word wider than width cut across lines.
func wrapped(value string, width int) []string {
	if width < 1 {
		return []string{value}
	}

	var lines []string
	line := ""
	for word := range strings.FieldsSeq(value) {
		switch {
		case line == "":
			line = word
		case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
		for lipgloss.Width(line) > width {
			lines = append(lines, ansi.Truncate(line, width, ""))
			line = ansi.TruncateLeft(line, width, "")
		}
	}
	return append(lines, line)
}

// Messages returns the message of each of warnings, for the Warnings of [PlanParams].
func Messages(warnings []domain.Warning) []string {
	texts := make([]string, 0, len(warnings))
	for _, warned := range warnings {
		texts = append(texts, warned.Message)
	}
	return texts
}

// DrawPlan writes the plan params describes to w, at the width of w's terminal and with its styles, whatever params says of them.
func DrawPlan(w io.Writer, params PlanParams) {
	params.Width = max(terminalWidth(w)-len(Indent), 0)
	params.Styles = StylesOf(w)

	for _, line := range PlanLines(params) {
		if line == "" {
			Blank(w)
			continue
		}
		Message(w, line)
	}
}
