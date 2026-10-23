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

// The marks of a plan's value: an answer recalled from memory, an answer the apply will remember, and one it remembered.
const (
	Recalled         = Dot + "remembered"
	WillBeRemembered = Dot + "will be remembered"
	Saved            = Dot + "saved for next runs"
)

// MarkText spells mark as a plan's value carries it, after a dot; nothing when there is no mark.
func MarkText(mark domain.Mark) string {
	switch mark {
	case domain.MarkRecalled:
		return Recalled
	case domain.MarkRemember:
		return WillBeRemembered
	case domain.MarkSaved:
		return Saved
	}

	return ""
}

// A PlanRow is one line of a plan: a label and the values it applies to.
type PlanRow struct {
	Label  string
	Values []string
}

// RecapRows are the rows of recap as a plan draws them: each value's text, the last carrying the row's mark.
func RecapRows(recap []domain.RecapRow) []PlanRow {
	rows := make([]PlanRow, 0, len(recap))
	for _, row := range recap {
		values := make([]string, 0, len(row.Values))
		for _, value := range row.Values {
			values = append(values, value.Text)
		}
		if last := len(values) - 1; last >= 0 {
			values[last] += MarkText(row.Mark)
		}
		rows = append(rows, PlanRow{Label: row.Label, Values: values})
	}

	return rows
}

// PlanParams is a plan to draw: its rows, the warnings under them, a muted hint closing it, and the width and styles to draw them with.
type PlanParams struct {
	Rows     []PlanRow
	Warnings []string
	Hint     string
	Width    int     // the columns a line may fill; 0 or less wraps nothing
	Styles   *Styles // nil draws no colour
}

// PlanLines returns the lines of the plan params describes: its rows with their values aligned in one column and wrapped to its width, then its warnings and its hint, each after an empty line.
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
	if params.Hint != "" {
		lines = append(lines, "", st.Muted.Render(params.Hint))
	}
	return lines
}

// RecalledHint closes a recap that answered a question from memory: how to be asked it again.
const RecalledHint = "Remembered answers are not asked: --ask asks them again."

// RecalledHintOf is RecalledHint when recalled names a field, else empty.
func RecalledHintOf(recalled []string) string {
	if len(recalled) == 0 {
		return ""
	}

	return RecalledHint
}

// MutedNote is note after a dot, muted, as a plan's value carries it; nothing when note is empty.
func MutedNote(note string, st *Styles) string {
	if note == "" {
		return ""
	}

	return st.Muted.Render(Dot + note)
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

// DryRunParams is a dry run to draw: the command, the rows of its plan, its warnings, its fingerprint, the line that applies exactly that plan, and how a warning's follow-up is spelled.
type DryRunParams struct {
	Command     string
	Rows        []PlanRow
	Warnings    []domain.Warning
	Fingerprint string
	Apply       string
	CommandLine CommandLine
}

// DryRun writes the dry run params describes to w: its title, its plan and warnings, its fingerprint, the line that applies it, then what its warnings offer.
func DryRun(w io.Writer, params DryRunParams) {
	SectionTitle(w, params.Command+Dot+"dry run, nothing applied")
	warnings := Messages(params.Warnings)
	DrawPlan(w, PlanParams{Rows: params.Rows, Warnings: warnings})
	if len(warnings) > 0 {
		Blank(w)
	}

	InfoLine(w, Info{Label: "fingerprint", Value: params.Fingerprint})
	NextSteps(w, []NextStep{{Command: params.Apply, Note: ApplyNote}})
	// a block of their own: aligned on the apply line, their notes would run past the screen
	NextSteps(w, WarningSteps(params.Warnings, params.CommandLine))
}
