package dashboard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// helpBox is the key and mouse reference: every clickable zone paired with the
// key that does the same thing. Like the other overlays it is a box pasted over
// the frame; unlike them it is a document, so what does not fit scrolls rather
// than being cut off the bottom of the screen.
func (m Model) helpBox() (string, domain.Rect) {
	layout := m.helpLayout()
	if layout.Inner <= 0 || layout.BodyRows <= 0 {
		return "", domain.Rect{}
	}

	content := helpContent(layout)
	offset := rules.DashboardClampOffset(rules.DashboardOffsetParams{
		Offset:  m.helpScroll,
		Total:   len(content),
		Visible: layout.BodyRows,
	})

	lines := []string{
		styles.DashboardModalTitle.Render(truncate(domain.DashboardHelpTitle, layout.Inner)),
		styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, layout.Inner)),
	}
	lines = append(lines, content[offset:offset+layout.BodyRows]...)
	lines = append(lines,
		styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, layout.Inner)),
		styles.DashboardModalHint.Render(truncate(helpHint(layout), layout.Inner)))

	box := styles.DashboardHelpBox.Width(layout.Inner + paddingWidth).Render(strings.Join(lines, "\n"))
	return box, rules.CenterRect(rules.CenterRectParams{
		Width:        lipgloss.Width(box),
		Height:       lipgloss.Height(box),
		ScreenWidth:  m.width,
		ScreenHeight: m.height,
	})
}

func (m Model) helpLayout() domain.HelpLayout {
	return rules.ComputeHelpLayout(rules.HelpLayoutParams{
		Sections:     rules.HelpSections(m.runModule()),
		ScreenWidth:  m.width,
		ScreenHeight: m.height,
	})
}

func helpHint(layout domain.HelpLayout) string {
	if layout.Scrollable {
		return domain.DashboardHelpHintScroll
	}
	return domain.DashboardHelpHint
}

// helpContent stacks the bands into the overlay's body. Sections in a band are
// drawn to the same height, so the next band's titles line up across columns.
func helpContent(layout domain.HelpLayout) []string {
	var lines []string
	for index, band := range layout.Bands {
		lines = append(lines, helpBandLines(layout, band)...)
		if index < len(layout.Bands)-1 {
			lines = append(lines, "")
		}
	}
	return lines
}

func helpBandLines(layout domain.HelpLayout, band domain.HelpBand) []string {
	titles := make([]string, len(band))
	underlines := make([]string, len(band))
	tallest := 0
	for column, section := range band {
		width := layout.ColumnWidth[column]
		titles[column] = styles.DashboardSectionTitle.Render(truncate(section.Title, width))
		underlines[column] = styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, width))
		tallest = max(tallest, len(section.Entries))
	}

	lines := []string{helpRow(layout, titles), helpRow(layout, underlines)}
	for row := range tallest {
		cells := make([]string, len(band))
		for column, section := range band {
			if row < len(section.Entries) {
				cells[column] = helpEntryLine(layout, column, section.Entries[row])
			}
		}
		lines = append(lines, helpRow(layout, cells))
	}
	return lines
}

func helpEntryLine(layout domain.HelpLayout, column int, entry domain.HelpEntry) string {
	rest := max(layout.ColumnWidth[column]-layout.KeyWidth[column], 0)
	keys := styles.DashboardHelpKey.Render(truncate(entry.Keys, layout.KeyWidth[column]))
	text := styles.DashboardValue.Render(truncate(entry.Text, rest))
	return pad(keys, layout.KeyWidth[column]) + text
}

// helpRow lays one row of cells across the columns. Every cell but the last is
// padded to its column: the last one carries no trailing run of spaces, which
// would otherwise paint the box's background past the text on that row.
func helpRow(layout domain.HelpLayout, cells []string) string {
	row := ""
	for column, cell := range cells {
		if column > 0 {
			row = pad(row, helpColumnStart(layout, column))
		}
		row += cell
	}
	return row
}

func helpColumnStart(layout domain.HelpLayout, column int) int {
	start := 0
	for index := range column {
		start += layout.ColumnWidth[index] + domain.DashboardHelpColumnGap
	}
	return start
}

// helpKey drives the reference while it is up. It documents "q · ctrl+c quit",
// so it must not swallow them; everything else it does not use is ignored
// rather than let through, because a reference is read, not acted from.
func (m Model) helpKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case keyQuit, keyInterrupt:
		return m.quit()
	case keyHelp, keyEscape:
		m.showHelp = false
	case keyUp, keyVimUp:
		return m.scrollHelp(-1), nil
	case keyDown, keyVimDown:
		return m.scrollHelp(1), nil
	case keyPageUp:
		return m.scrollHelp(-m.helpLayout().BodyRows), nil
	case keyPageDown:
		return m.scrollHelp(m.helpLayout().BodyRows), nil
	case keyTop:
		m.helpScroll = 0
	case keyBottom:
		return m.scrollHelp(m.helpLayout().ContentRows), nil
	}
	return m, nil
}

// helpMouse gives the reference the mouse it documents: the wheel scrolls it,
// and a click beside it dismisses it the way clicking beside a menu does.
func (m Model) helpMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.scrollHelp(-1), nil
	case tea.MouseButtonWheelDown:
		return m.scrollHelp(1), nil
	}
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if _, rect := m.helpBox(); !inRect(rect, msg) {
		m.showHelp = false
	}
	return m, nil
}

func (m Model) scrollHelp(delta int) Model {
	layout := m.helpLayout()
	m.helpScroll = rules.DashboardClampOffset(rules.DashboardOffsetParams{
		Offset:  m.helpScroll + delta,
		Total:   layout.ContentRows,
		Visible: layout.BodyRows,
	})
	return m
}

func inRect(rect domain.Rect, msg tea.MouseMsg) bool {
	return msg.X >= rect.X && msg.X < rect.X+rect.Width &&
		msg.Y >= rect.Y && msg.Y < rect.Y+rect.Height
}
