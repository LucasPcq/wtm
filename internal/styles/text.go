package styles

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Indent is the standard left-padding applied to all formatted output and TUI
// components. Centralised here so both output/ and tui/ share the same value.
const Indent = "  "

var (
	// Bold renders text in bold.
	Bold = lipgloss.NewStyle().Bold(true)

	// Muted renders deemphasized text.
	Muted = lipgloss.NewStyle().Foreground(ColorMuted)

	// Success renders positive-state text.
	Success = lipgloss.NewStyle().Foreground(ColorSuccess)

	// Warning renders attention-needed text.
	Warning = lipgloss.NewStyle().Foreground(ColorWarning)

	// Primary renders accent text.
	Primary = lipgloss.NewStyle().Foreground(ColorPrimary)
)

type NextStepParams struct {
	Command string
	// Note glosses the command. It is chrome — an annotation, never content — so
	// it is the one part of the line that is muted.
	Note string
}

// NextStepLine composes the one forward-pointing line of a conclusion: the
// arrow, the command in bold, and an optional muted gloss. It lives here rather
// than in output/ because tui/ concludes on the same line — a run recap printed
// once the view gives the terminal back — and tui/ may not import output/.
func NextStepLine(params NextStepParams) string {
	line := Bold.Render(params.Command)
	if params.Note != "" {
		line += Muted.Render(domain.NextStepNoteSeparator + params.Note)
	}
	return Indent + Primary.Render(domain.NextStepGlyph) + " " + line
}
