package output

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#5B4BE0", Dark: "#9B8CFF"}
	ColorMuted   = lipgloss.AdaptiveColor{Light: "#6F6F6F", Dark: "#8D8D8D"}
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#158A4A", Dark: "#4ADE80"}
	ColorWarning = lipgloss.AdaptiveColor{Light: "#B07800", Dark: "#F0C000"}
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#C21E28", Dark: "#FA6E76"}

	bold    = lipgloss.NewStyle().Bold(true)
	muted   = lipgloss.NewStyle().Foreground(ColorMuted)
	success = lipgloss.NewStyle().Foreground(ColorSuccess)
	warning = lipgloss.NewStyle().Foreground(ColorWarning)
	primary = lipgloss.NewStyle().Foreground(ColorPrimary)
	danger  = lipgloss.NewStyle().Foreground(ColorDanger)
)
