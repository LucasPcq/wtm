package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
	"github.com/charmbracelet/lipgloss"
)

// HelpSeparator joins the parts of the breadcrumb and of the help line.
const (
	HelpSeparator = " • "
)

// styles are the styles of a form: those of its widgets, and the wizard's own, rendered with the colour profile of the stream it draws on.
type styles struct {
	*widget.Styles
	summary   lipgloss.Style
	statusBox lipgloss.Style
	recapBox  lipgloss.Style
}

func newStyles(base *output.Styles) *styles {
	return &styles{
		Styles:    widget.NewStyles(base),
		summary:   base.NewStyle().Foreground(output.ColorMuted).Italic(true),
		statusBox: base.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(output.ColorMuted).Padding(0, 1).MarginLeft(2),
		recapBox:  base.NewStyle().Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(output.ColorPrimary).PaddingLeft(2).MarginLeft(2),
	}
}

// Indented returns text with each of its lines indented like the rest of wtm's output.
func Indented(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = output.Indent + line
	}
	return strings.Join(lines, "\n")
}
