package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

type overlayParams struct {
	Base string
	Box  string
	At   domain.Rect
}

// overlay pastes a box over the frame, cell by cell, so what is underneath stays
// on screen around it.
//
// The frame it cuts through must carry no zone marker: a marker is an escape
// sequence, and cutting a line at a column would drop or split it, taking the
// hit-testing of whatever it marked with it. That is what marks() is for — under
// an overlay the frame is drawn unmarked, and only the box marks anything.
func overlay(params overlayParams) string {
	lines := strings.Split(params.Base, "\n")
	box := strings.Split(params.Box, "\n")
	width := lipgloss.Width(params.Box)

	for index, boxLine := range box {
		y := params.At.Y + index
		if y < 0 || y >= len(lines) {
			break
		}
		lines[y] = pasteRow(pasteParams{Line: lines[y], Box: boxLine, X: params.At.X, Width: width})
	}
	return strings.Join(lines, "\n")
}

type pasteParams struct {
	Line  string
	Box   string
	X     int
	Width int
}

func pasteRow(params pasteParams) string {
	left := ansi.Truncate(params.Line, params.X, "")
	if gap := params.X - ansi.StringWidth(left); gap > 0 {
		left += strings.Repeat(" ", gap)
	}
	box := params.Box
	if gap := params.Width - lipgloss.Width(box); gap > 0 {
		box += strings.Repeat(" ", gap)
	}
	// No reset between the two: the box's own styling closes itself, and
	// TruncateLeft re-emits the style the right side was under.
	return left + box + ansi.TruncateLeft(params.Line, params.X+params.Width, "")
}

// noMarks is the marker of a frame nothing may be clicked in, because an overlay
// has the mouse.
type noMarks struct{}

func (noMarks) Mark(_, content string) string { return content }

// marks reports where the frame's clickable regions are registered: nowhere,
// while an overlay is up.
func (m Model) marks() marker {
	if m.modal.open || m.menuOpen || m.showHelp {
		return noMarks{}
	}
	return m.zones
}
