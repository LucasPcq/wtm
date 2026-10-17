package form

import (
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	HelpSeparator = " • "
)

var (
	colorRowTint    = lipgloss.AdaptiveColor{Light: "#EFEBFD", Dark: "#282341"}
	colorSelectedBg = lipgloss.AdaptiveColor{Light: "#DCD7FB", Dark: "#3B2FA8"}
	colorSelectedFg = lipgloss.AdaptiveColor{Light: "#1A1150", Dark: "#FFFFFF"}
)

type Styles struct {
	*output.Styles
	summary   lipgloss.Style
	accent    lipgloss.Style
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
		tinted:    base.NewStyle().Background(colorRowTint).Foreground(colorSelectedFg),
		selected:  base.NewStyle().Background(colorSelectedBg).Foreground(colorSelectedFg),
		reverse:   base.NewStyle().Reverse(true),
		statusBox: base.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(output.ColorMuted).Padding(0, 1).MarginLeft(2),
		recapBox:  base.NewStyle().Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(output.ColorPrimary).PaddingLeft(2).MarginLeft(2),
	}
}

func Wrap(value string, width int) string {
	if width < 1 {
		return value
	}
	return ansi.Wrap(value, width, "")
}

func Indented(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = output.Indent + line
	}
	return strings.Join(lines, "\n")
}

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
