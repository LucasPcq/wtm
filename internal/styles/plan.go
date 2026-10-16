package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

const (
	planGap          = "  "
	planContinuation = "  "
	planSeparator    = ", "
	nothingToDo      = "nothing to do"
)

// PlanRow is one line of a plan as every surface shows it: what the plan sets,
// and to what — one value, or the items of a list.
type PlanRow struct {
	Label  string
	Values []string
}

type PlanParams struct {
	Rows     []PlanRow
	Warnings []string
	// Width is the columns the plan may take; zero leaves every line whole.
	Width int
}

// RenderPlan is the plan of a command as the command line's dry run and the
// form's recap both draw it: its rows, labels aligned, then its warnings apart.
// A list that does not fit on its line takes a line per item, and a long value
// wraps, both under the column of the values.
func RenderPlan(params PlanParams) string {
	lines := planRows(params)
	if len(params.Warnings) > 0 {
		lines = append(lines, "")
	}
	for _, warning := range params.Warnings {
		first, rest, _ := strings.Cut(warning, "\n")
		lines = append(lines, Warning.Render(domain.GlyphAttention)+" "+first)
		for _, line := range strings.Split(rest, "\n") {
			if line != "" {
				lines = append(lines, planContinuation+line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func planRows(params PlanParams) []string {
	if len(params.Rows) == 0 {
		return []string{Muted.Render(domain.GlyphUnchanged + " " + nothingToDo)}
	}
	labels := 0
	for _, row := range params.Rows {
		labels = max(labels, lipgloss.Width(row.Label))
	}
	column := labels + len(planGap)
	var lines []string
	for _, row := range params.Rows {
		values := planValues(row.Values, params.Width-column)
		lines = append(lines, Muted.Render(row.Label)+strings.Repeat(" ", column-lipgloss.Width(row.Label))+values[0])
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

// wrapped breaks a value at its spaces only: a branch or a path is never cut
// at a hyphen, unless it is longer than the line on its own.
func wrapped(value string, width int) []string {
	if width < 1 {
		return []string{value}
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(value) {
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
