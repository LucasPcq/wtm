package dashboard

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	foldGlyphCollapsed = "▸"
	foldGlyphExpanded  = "▾"
)

// renderOutput draws the permanent bottom panel. It is wired but silent: the
// operation flows fill it through OutputLineMsg.
func (m Model) renderOutput(layout domain.DashboardLayout) string {
	width := layout.Output.Width - borderWidth - paddingWidth
	if width <= 0 || layout.Output.Height <= 0 {
		return ""
	}

	glyph := foldGlyphCollapsed
	if m.outputExpanded {
		glyph = foldGlyphExpanded
	}

	return m.renderPanel(panelParams{
		Rect:      layout.Output,
		Title:     glyph + " " + domain.DashboardOutputTitle,
		TitleZone: zoneOutputToggle,
		Body:      m.outputBody(layout, width),
		Zone:      zoneOutput,
	})
}

func (m Model) outputBody(layout domain.DashboardLayout, width int) []string {
	if layout.OutputLines <= 0 {
		return nil
	}
	if len(m.outputLines) == 0 {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptyOutput, width))}
	}

	end := min(m.outputOffset+layout.OutputLines, len(m.outputLines))
	lines := make([]string, 0, max(end-m.outputOffset, 0))
	for index := m.outputOffset; index < end; index++ {
		lines = append(lines, truncate(m.outputLines[index], width))
	}
	return lines
}
