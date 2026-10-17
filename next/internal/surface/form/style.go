package form

import (
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/next/internal/surface/output"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	indent         = "  "
	glyphFailure   = "✗"
	glyphAttention = "!"
	helpSeparator  = " • "
)

var (
	colorRowTint    = lipgloss.AdaptiveColor{Light: "#EFEBFD", Dark: "#282341"}
	colorSelectedBg = lipgloss.AdaptiveColor{Light: "#DCD7FB", Dark: "#3B2FA8"}
	colorSelectedFg = lipgloss.AdaptiveColor{Light: "#1A1150", Dark: "#FFFFFF"}

	muted     = lipgloss.NewStyle().Foreground(output.ColorMuted)
	summary   = lipgloss.NewStyle().Foreground(output.ColorMuted).Italic(true)
	accent    = lipgloss.NewStyle().Foreground(output.ColorPrimary).Bold(true)
	danger    = lipgloss.NewStyle().Foreground(output.ColorDanger)
	warning   = lipgloss.NewStyle().Foreground(output.ColorWarning)
	tinted    = lipgloss.NewStyle().Background(colorRowTint).Foreground(colorSelectedFg)
	selected  = lipgloss.NewStyle().Background(colorSelectedBg).Foreground(colorSelectedFg)
	reverse   = lipgloss.NewStyle().Reverse(true)
	statusBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(output.ColorMuted).Padding(0, 1).MarginLeft(2)
	recapBox  = lipgloss.NewStyle().Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(output.ColorPrimary).PaddingLeft(2).MarginLeft(2)
)

func wrap(value string, width int) string {
	if width < 1 {
		return value
	}
	return ansi.Wrap(value, width, "")
}

func indented(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func errorBanner(message string, width int) string {
	hang := indent + "  "
	lines := strings.Split(wrap(capitalized(message), width-len(hang)-1), "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = hang + danger.Render(lines[i])
	}
	lines[0] = indent + danger.Render(glyphFailure) + " " + danger.Render(lines[0])
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
