package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// CmdListModel amends the command of a job whose port variable it never
// mentions. wtm injects the variable; only the command can decide to read it,
// and this is the last moment before the config is written where that costs a
// keystroke rather than another command.
type CmdListModel struct {
	fixes   []domain.JobCmdFix
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

type NewCmdListParams struct {
	Title       string
	Description string
	Fixes       []domain.JobCmdFix
}

func NewCmdList(params NewCmdListParams) CmdListModel {
	return CmdListModel{
		fixes: params.Fixes,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
}

func (m CmdListModel) Fixes() []domain.JobCmdFix { return m.fixes }
func (m CmdListModel) Done() bool                { return m.done }
func (m CmdListModel) Aborted() bool             { return m.aborted }
func (m CmdListModel) Init() tea.Cmd             { return nil }

func (m *CmdListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m CmdListModel) doneRow() int { return len(m.fixes) }

func (m CmdListModel) Update(msg tea.Msg) (CmdListModel, tea.Cmd) {
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
		return m.startEdit(), nil
	case "esc":
		m.aborted = true
	}

	return m.scrolled(), nil
}

// startEdit opens on the current command: it is amended, not retyped.
func (m CmdListModel) startEdit() CmdListModel {
	input := textinput.New()
	input.CharLimit = domain.CmdListCharLimit
	input.Width = max(domain.CmdListMinWidth, m.width-domain.CmdListWidthInset)
	input.SetValue(m.fixes[m.cursor].Cmd)
	input.CursorEnd()
	input.Focus()

	m.input = input
	m.editing = true
	m.err = ""
	return m
}

func (m CmdListModel) updateEdit(msg tea.Msg) (CmdListModel, tea.Cmd) {
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

func (m CmdListModel) saveEdit() CmdListModel {
	cmd := strings.TrimSpace(m.input.Value())
	if cmd == "" {
		m.err = domain.CmdListEmptyErr
		return m
	}

	m.fixes[m.cursor].Cmd = cmd
	m.editing = false
	m.err = ""
	return m
}

func (m CmdListModel) View() string {
	body, _ := windowBody(m.window())
	return body
}

func (m CmdListModel) window() bodyWindowParams {
	jobWidth, varsWidth := rules.CmdFixWidths(m.fixes)

	rows := make([]string, 0, len(m.fixes)+1)
	for i, fix := range m.fixes {
		label := rules.CmdFixLabel(fix, jobWidth, varsWidth)
		if m.editing && i == m.cursor {
			label = fmt.Sprintf(domain.CmdListEditFmt, fix.Job, strings.Join(fix.Vars, domain.CmdListVarSep), m.input.View())
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
func (m CmdListModel) scrolled() CmdListModel {
	_, m.offset = windowBody(m.window())
	return m
}

func (m CmdListModel) helpActions() []string { return nil }

func (m CmdListModel) helpModal() string {
	if m.editing {
		return domain.CmdListEditHelp
	}
	return ""
}

func (m CmdListModel) renderRow(label string, selected bool) string {
	if selected {
		line := "▸ " + label
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		return styles.ListItemSelected.Render(line)
	}
	return styles.ListItemNormal.Render(styles.Indent + label)
}
