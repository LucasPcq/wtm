package output

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.AdaptiveColor{Light: "#5B4BE0", Dark: "#9B8CFF"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#6F6F6F", Dark: "#8D8D8D"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "#158A4A", Dark: "#4ADE80"}
	colorWarning = lipgloss.AdaptiveColor{Light: "#B07800", Dark: "#F0C000"}
	colorDanger  = lipgloss.AdaptiveColor{Light: "#C21E28", Dark: "#FA6E76"}

	bold    = lipgloss.NewStyle().Bold(true)
	muted   = lipgloss.NewStyle().Foreground(colorMuted)
	success = lipgloss.NewStyle().Foreground(colorSuccess)
	warning = lipgloss.NewStyle().Foreground(colorWarning)
	primary = lipgloss.NewStyle().Foreground(colorPrimary)
	danger  = lipgloss.NewStyle().Foreground(colorDanger)
)
