package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// ReorderItem represents a movable entry in a ReorderList.
type ReorderItem struct {
	Label string
	Value string
}

// ReorderListModel is an ordered list where the focused item can be moved up or
// down with shift+↑/↓ (or K/J). The slice order is the result.
type ReorderListModel struct {
	items   []ReorderItem
	cursor  int
	width   int
	height  int
	offset  int
	title   string
	desc    string
	done    bool
	aborted bool
}

// NewReorderListParams holds inputs for NewReorderList.
type NewReorderListParams struct {
	Title       string
	Description string
	Items       []ReorderItem
}

// NewReorderList creates a ReorderListModel.
func NewReorderList(params NewReorderListParams) ReorderListModel {
	return ReorderListModel{
		items: params.Items,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
}

// Done returns true after the user confirmed.
func (m ReorderListModel) Done() bool { return m.done }

// Aborted returns true after the user pressed Esc.
func (m ReorderListModel) Aborted() bool { return m.aborted }

// Values returns the item values in their current order.
func (m ReorderListModel) Values() []string {
	vals := make([]string, 0, len(m.items))
	for _, item := range m.items {
		vals = append(vals, item.Value)
	}
	return vals
}

// Init satisfies tea.Model.
func (m ReorderListModel) Init() tea.Cmd { return nil }

// Update handles key events.
func (m ReorderListModel) Update(msg tea.Msg) (ReorderListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "shift+up", "K":
		if m.cursor > 0 {
			m.items[m.cursor], m.items[m.cursor-1] = m.items[m.cursor-1], m.items[m.cursor]
			m.cursor--
		}
	case "shift+down", "J":
		if m.cursor < len(m.items)-1 {
			m.items[m.cursor], m.items[m.cursor+1] = m.items[m.cursor+1], m.items[m.cursor]
			m.cursor++
		}
	case "enter":
		m.done = true
	case "esc":
		m.aborted = true
	}

	m.clampOffset()
	return m, nil
}

// View renders the numbered, reorderable list.
func (m ReorderListModel) helpActions() []string { return []string{domain.HelpReorder} }

func (m ReorderListModel) helpModal() string { return "" }

func (m ReorderListModel) View() string {
	if len(m.items) == 0 {
		return styles.Muted.Render("  No items")
	}

	var b strings.Builder

	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		visibleHeight = len(m.items)
	}

	end := m.offset + visibleHeight
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.offset; i < end; i++ {
		item := m.items[i]
		label := fmt.Sprintf("%d. %s", i+1, item.Label)

		if i == m.cursor {
			line := "▸ " + label
			pad := m.width - len(line)
			if pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			b.WriteString(styles.ListItemSelected.Render(line))
		} else {
			b.WriteString(styles.ListItemNormal.Render(styles.Indent + label))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m ReorderListModel) visibleHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(1, m.height)
}

func (m *ReorderListModel) clampOffset() {
	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}
