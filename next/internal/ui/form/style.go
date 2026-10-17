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

	Summary   = lipgloss.NewStyle().Foreground(output.ColorMuted).Italic(true)
	Accent    = lipgloss.NewStyle().Foreground(output.ColorPrimary).Bold(true)
	tinted    = lipgloss.NewStyle().Background(colorRowTint).Foreground(colorSelectedFg)
	selected  = lipgloss.NewStyle().Background(colorSelectedBg).Foreground(colorSelectedFg)
	reverse   = lipgloss.NewStyle().Reverse(true)
	StatusBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(output.ColorMuted).Padding(0, 1).MarginLeft(2)
	RecapBox  = lipgloss.NewStyle().Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(output.ColorPrimary).PaddingLeft(2).MarginLeft(2)
)

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

func errorBanner(message string, width int) string {
	hang := output.Indent + "  "
	lines := strings.Split(Wrap(capitalized(message), width-len(hang)-1), "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = hang + output.Danger.Render(lines[i])
	}
	lines[0] = output.Indent + output.Danger.Render(output.GlyphFailure) + " " + output.Danger.Render(lines[0])
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
