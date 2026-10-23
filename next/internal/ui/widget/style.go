package widget

import (
	"errors"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	colorRowTint    = lipgloss.AdaptiveColor{Light: "#EFEBFD", Dark: "#282341"}
	colorSelectedFg = lipgloss.AdaptiveColor{Light: "#1A1150", Dark: "#FFFFFF"}
)

// Styles are the styles of the widgets: those of the stream they draw on, and their own, rendered with that stream's colour profile.
type Styles struct {
	*output.Styles
	Accent  lipgloss.Style // a title or the row the cursor is on
	success lipgloss.Style
	tinted  lipgloss.Style
	reverse lipgloss.Style
}

// NewStyles returns the widgets' styles over base, the styles of the stream they draw on.
func NewStyles(base *output.Styles) *Styles {
	return &Styles{
		Styles:  base,
		Accent:  base.NewStyle().Foreground(output.ColorPrimary).Bold(true),
		success: base.NewStyle().Foreground(output.ColorSuccess),
		tinted:  base.NewStyle().Background(colorRowTint).Foreground(colorSelectedFg),
		reverse: base.NewStyle().Reverse(true),
	}
}

// Wrap returns value wrapped to width columns, escape codes aside, or value as it is when width is under 1.
func Wrap(value string, width int) string {
	if width < 1 {
		return value
	}
	return ansi.Wrap(value, width, "")
}

// A Problem is an error a widget shows followed by the steps that get past it.
type Problem struct {
	Message string
	Next    []output.NextStep
}

// Error is the problem's message.
func (p *Problem) Error() string {
	return p.Message
}

// errorBanner draws err as a widget's error, wrapped to width under the failure glyph, then the steps of a *Problem.
func errorBanner(err error, width int, st *Styles) string {
	hang := output.Indent + "  "
	lines := strings.Split(Wrap(err.Error(), width-len(hang)-1), "\n")

	for i := 1; i < len(lines); i++ {
		lines[i] = hang + st.Danger.Render(lines[i])
	}
	lines[0] = output.Indent + st.Danger.Render(output.GlyphFailure) + " " + st.Danger.Render(lines[0])
	if problem, ok := errors.AsType[*Problem](err); ok {
		lines = append(lines, output.NextStepLines(problem.Next, st.Styles)...)
	}
	return strings.Join(lines, "\n")
}
