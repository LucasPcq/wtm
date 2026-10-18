package output

import (
	"io"

	"github.com/charmbracelet/lipgloss"
)

// The palette of wtm's output, each colour with a light and a dark variant the terminal's background picks from.
var (
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#5B4BE0", Dark: "#9B8CFF"}
	ColorMuted   = lipgloss.AdaptiveColor{Light: "#6F6F6F", Dark: "#8D8D8D"}
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#158A4A", Dark: "#4ADE80"}
	ColorWarning = lipgloss.AdaptiveColor{Light: "#B07800", Dark: "#F0C000"}
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#C21E28", Dark: "#FA6E76"}
)

// Styles are the styles of one stream, rendered with that stream's colour profile so that a pipe gets no escape codes.
type Styles struct {
	renderer *lipgloss.Renderer
	bold     lipgloss.Style
	Muted    lipgloss.Style // chrome and lines that report no event
	Success  lipgloss.Style // a glyph of something done
	Warning  lipgloss.Style
	primary  lipgloss.Style
	Danger   lipgloss.Style
}

var plainStyles = newStyles(lipgloss.NewRenderer(io.Discard))

func newStyles(r *lipgloss.Renderer) *Styles {
	return &Styles{
		renderer: r,
		bold:     r.NewStyle().Bold(true),
		Muted:    r.NewStyle().Foreground(ColorMuted),
		Success:  r.NewStyle().Foreground(ColorSuccess),
		Warning:  r.NewStyle().Foreground(ColorWarning),
		primary:  r.NewStyle().Foreground(ColorPrimary),
		Danger:   r.NewStyle().Foreground(ColorDanger),
	}
}

// NewStyle returns an empty style rendered with the stream's colour profile, for a style the kit does not hold.
func (s *Styles) NewStyle() lipgloss.Style {
	return s.renderer.NewStyle()
}
