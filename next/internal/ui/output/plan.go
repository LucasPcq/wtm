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
	NothingToDo      = "nothing to do"
)

type PlanRow struct {
	Label  string
	Values []string
}

type PlanParams struct {
	Rows     []PlanRow
	Warnings []string
	Width    int
	Styles   *Styles
}

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

func Messages(warnings []domain.Warning) []string {
	texts := make([]string, 0, len(warnings))
	for _, warned := range warnings {
		texts = append(texts, warned.Message)
	}
	return texts
}

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
