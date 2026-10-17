package form

import (
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// HelpSeparator joins the parts of the breadcrumb and of the help line.
const (
	HelpSeparator = " • "
)

var (
	colorRowTint    = lipgloss.AdaptiveColor{Light: "#EFEBFD", Dark: "#282341"}
	colorSelectedBg = lipgloss.AdaptiveColor{Light: "#DCD7FB", Dark: "#3B2FA8"}
	colorSelectedFg = lipgloss.AdaptiveColor{Light: "#1A1150", Dark: "#FFFFFF"}
)

// Styles are the styles of a form: those of the stream it draws on, and its own, rendered with that stream's colour profile.
type Styles struct {
	*output.Styles
	summary   lipgloss.Style
	accent    lipgloss.Style
	success   lipgloss.Style
	tinted    lipgloss.Style
	selected  lipgloss.Style
	reverse   lipgloss.Style
	statusBox lipgloss.Style
	recapBox  lipgloss.Style
}

func newStyles(base *output.Styles) *Styles {
	return &Styles{
		Styles:    base,
		summary:   base.NewStyle().Foreground(output.ColorMuted).Italic(true),
		accent:    base.NewStyle().Foreground(output.ColorPrimary).Bold(true),
		success:   base.NewStyle().Foreground(output.ColorSuccess),
		tinted:    base.NewStyle().Background(colorRowTint).Foreground(colorSelectedFg),
		selected:  base.NewStyle().Background(colorSelectedBg).Foreground(colorSelectedFg),
		reverse:   base.NewStyle().Reverse(true),
		statusBox: base.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(output.ColorMuted).Padding(0, 1).MarginLeft(2),
		recapBox:  base.NewStyle().Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(output.ColorPrimary).PaddingLeft(2).MarginLeft(2),
	}
}

// Wrap returns value wrapped to width columns, escape codes aside, or value as it is when width is under 1.
func Wrap(value string, width int) string {
	if width < 1 {
		return value
	}
	return ansi.Wrap(value, width, "")
}

// Indented returns text with each of its lines indented like the rest of wtm's output.
func Indented(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = output.Indent + line
	}
	return strings.Join(lines, "\n")
}

// errorBanner draws message as a field's error, wrapped to width under the failure glyph.
func errorBanner(message string, width int, st *Styles) string {
	hang := output.Indent + "  "
	lines := strings.Split(Wrap(capitalized(message), width-len(hang)-1), "\n")

	for i := 1; i < len(lines); i++ {
		lines[i] = hang + st.Danger.Render(lines[i])
	}
	lines[0] = output.Indent + st.Danger.Render(output.GlyphFailure) + " " + st.Danger.Render(lines[0])
	return strings.Join(lines, "\n")
}

func capitalized(text string) string {
	if text == "" {
		return text
	}
	runes := []rune(text)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
