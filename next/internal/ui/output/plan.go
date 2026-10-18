package output

import (
	"cmp"
	"io"
	"slices"
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

// Marks say which answers of a plan are marked, by the path of their field: those recalled from memory, and those remembered for the next runs with the mark that says so, WillBeRemembered before the apply, Saved after it.
type Marks struct {
	Recalled   []string
	Remembered []string
	Mark       string
}

// Of is the mark of the answer to the field at path, empty when it has none.
func (m Marks) Of(path string) string {
	switch {
	case slices.Contains(m.Remembered, path):
		return m.Mark
	case slices.Contains(m.Recalled, path):
		return Recalled
	}

	return ""
}

// EnvValueParams is the env strategy of a plan: the strategy, whether it follows config.toml's, and its mark.
type EnvValueParams struct {
	Strategy      string
	FollowsConfig bool
	Mark          string
}

// EnvValue spells the env strategy with its mark; a marked answer that follows config.toml's reads config default, since that, not the strategy, is what is remembered.
func EnvValue(params EnvValueParams) string {
	if params.FollowsConfig && params.Mark != "" {
		return "config default (" + params.Strategy + ")" + params.Mark
	}

	return params.Strategy + params.Mark
}

// A PlanRow is one line of a plan: a label and the values it applies to.
type PlanRow struct {
	Label  string
	Values []string
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

// NotedParams are the values of a plan's row, the note the row carries and the value it shows when it has none but a note.
type NotedParams struct {
	Values []string
	Note   string
	None   string
}

// Noted are the values of params, the last carrying its note; its None, noted, when there are no values but a note; none when there is neither.
func Noted(params NotedParams) []string {
	switch {
	case params.Note == "":
		return params.Values
	case len(params.Values) == 0:
		return []string{params.None + params.Note}
	}

	last := len(params.Values) - 1
	return append(slices.Clone(params.Values[:last]), params.Values[last]+params.Note)
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

// DryRunParams is a dry run to draw: the command, the rows of its plan, its warnings, its fingerprint and the line that applies exactly that plan.
type DryRunParams struct {
	Command     string
	Rows        []PlanRow
	Warnings    []domain.Warning
	Fingerprint string
	Apply       string
}

// DryRun writes the dry run params describes to w: its title, its plan and warnings, its fingerprint and the line that applies it.
func DryRun(w io.Writer, params DryRunParams) {
	SectionTitle(w, params.Command+Dot+"dry run, nothing applied")
	warnings := Messages(params.Warnings)
	DrawPlan(w, PlanParams{Rows: params.Rows, Warnings: warnings})
	if len(warnings) > 0 {
		Blank(w)
	}

	InfoLine(w, Info{Label: "fingerprint", Value: params.Fingerprint})
	NextSteps(w, []NextStep{{Command: params.Apply, Note: ApplyNote}})
}
