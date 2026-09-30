package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	portInputCharLimit = 5
	portInputWidth     = 12
)

// PortListModel reviews the ports detection pre-filled. Enter on a row edits its
// base, Enter on the Done row confirms — the same shape as HookListModel, so a
// reader who knows one knows the other.
type PortListModel struct {
	entries []domain.PortEntry
	cursor  int
	offset  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool

	editing bool
	input   textinput.Model
	err     string
}

type NewPortListParams struct {
	Title       string
	Description string
	Entries     []domain.PortEntry
}

func NewPortList(params NewPortListParams) PortListModel {
	return PortListModel{
		entries: params.Entries,
		title:   params.Title,
		desc:    params.Description,
		width:   80,
	}
}

func (m PortListModel) Entries() []domain.PortEntry { return m.entries }
func (m PortListModel) Done() bool                  { return m.done }
func (m PortListModel) Aborted() bool               { return m.aborted }
func (m PortListModel) Init() tea.Cmd               { return nil }

func (m *PortListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m PortListModel) doneRow() int { return len(m.entries) }

func (m PortListModel) Update(msg tea.Msg) (PortListModel, tea.Cmd) {
	if m.editing {
		return m.updateEdit(msg)
	}

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
		if m.cursor < m.doneRow() {
			m.cursor++
		}
	case "enter":
		if m.cursor == m.doneRow() {
			m.done = true
			return m, nil
		}
		if m.entries[m.cursor].BindsNone {
			return m.declarePort(), nil
		}
		return m.startEdit(), nil
	case "n":
		if m.cursor < m.doneRow() && m.entries[m.cursor].CanBindNone {
			return m.toggleBindsNothing(), nil
		}
	case "esc":
		m.aborted = true
	}

	return m.scrolled(), nil
}

// toggleBindsNothing answers the row, and un-answers it: the same key both
// ways, because a reader who just pressed it looks for it again to undo it.
func (m PortListModel) toggleBindsNothing() PortListModel {
	entries := make([]domain.PortEntry, len(m.entries))
	copy(entries, m.entries)
	entries[m.cursor].BindsNone = !entries[m.cursor].BindsNone
	if entries[m.cursor].BindsNone {
		entries[m.cursor].Base = 0
	}
	m.entries = entries
	return m
}

// declarePort is enter on an answered row: it takes the question back and opens
// the edit in one gesture, so the answer is never a dead end.
func (m PortListModel) declarePort() PortListModel {
	entries := make([]domain.PortEntry, len(m.entries))
	copy(entries, m.entries)
	entries[m.cursor].BindsNone = false
	m.entries = entries
	return m.startEdit()
}

func (m PortListModel) startEdit() PortListModel {
	input := textinput.New()
	input.CharLimit = portInputCharLimit
	input.Width = portInputWidth
	// Empty, with the detected port as placeholder: a port is retyped, never
	// edited character by character, and an empty entry keeps what was detected.
	if base := m.entries[m.cursor].Base; base > 0 {
		input.Placeholder = strconv.Itoa(base)
	}
	input.Focus()

	m.input = input
	m.editing = true
	m.err = ""
	return m
}

func (m PortListModel) updateEdit(msg tea.Msg) (PortListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "enter":
		return m.saveEdit(), nil
	case "esc":
		m.editing = false
		m.err = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// saveEdit refuses anything that is not a usable port rather than overwriting a
// detected value that works.
func (m PortListModel) saveEdit() PortListModel {
	raw := strings.TrimSpace(m.input.Value())
	if raw == "" {
		m.editing = false
		m.err = ""
		return m
	}

	port, err := strconv.Atoi(raw)
	if err != nil || port < domain.PortMin || port > domain.PortMax {
		m.err = fmt.Sprintf(domain.PortListRangeErrFmt, raw, domain.PortMin, domain.PortMax)
		return m
	}

	m.entries[m.cursor].Base = port
	m.editing = false
	m.err = ""
	return m
}

func (m PortListModel) View() string {
	body, _ := windowBody(m.window())
	return body
}

func (m PortListModel) window() bodyWindowParams {
	jobWidth, nameWidth := rules.PortEntryWidths(m.entries)

	rows := make([]string, 0, len(m.entries)+1)
	for i, entry := range m.entries {
		label := rules.PortEntryLabel(entry, jobWidth, nameWidth)
		if m.editing && i == m.cursor {
			label = rules.PortEntryEditLabel(entry, m.input.View(), jobWidth, nameWidth)
		}
		rows = append(rows, m.renderRow(label, i == m.cursor))
	}
	rows = append(rows, m.renderRow(domain.WizardDoneRow, m.cursor == m.doneRow()))

	extras := ""
	if m.err != "" {
		extras = "\n\n" + errorBanner(m.err)
	}
	return bodyWindowParams{
		Rows: rows, Offset: m.offset, Height: m.height,
		Top: m.cursor, Bottom: m.cursor, Extras: extras,
	}
}

// scrolled settles where the window sits after the cursor moved, so the next
// render scrolls from there rather than snapping back to the top of the list.
func (m PortListModel) scrolled() PortListModel {
	_, m.offset = windowBody(m.window())
	return m
}

func (m PortListModel) helpActions() []string { return []string{domain.HelpBindsNoPort} }

func (m PortListModel) helpModal() string {
	if m.editing {
		return domain.PortListEditHelp
	}
	return ""
}

func (m PortListModel) renderRow(label string, selected bool) string {
	if selected {
		line := "▸ " + label
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		return styles.ListItemSelected.Render(line)
	}
	return styles.ListItemNormal.Render(styles.Indent + label)
}
