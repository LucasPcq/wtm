package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	// hookInputCharLimit caps the cmd/cwd text inputs in the inline hook form.
	hookInputCharLimit = 256

	// hookInputWidthInset is the column budget reserved around the inline form
	// inputs so they don't overrun the wizard frame.
	hookInputWidthInset = 24

	// hookInputMinWidth keeps the inputs usable when the terminal is narrow.
	hookInputMinWidth = 10

	// hookFormFieldCount is the number of focusable fields in the edit form:
	// cmd, cwd, and the continue_on_error toggle. Drives focus wrap-around.
	hookFormFieldCount = 3
)

// HookListModel edits an ordered list of on_create hooks. Browse mode lists the
// hooks plus "+ Add a command" and "✓ Done" rows; edit mode is an inline form
// with cmd, cwd, and a continue_on_error toggle.
type HookListModel struct {
	hooks   []domain.HookCommand
	cursor  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool

	editing   bool
	editIndex int // index being edited, or -1 when adding
	cmdInput  textinput.Model
	cwdInput  textinput.Model
	contOnErr bool
	formFocus int // 0=cmd, 1=cwd, 2=continue toggle
	formErr   string
}

// NewHookListParams holds inputs for NewHookList.
type NewHookListParams struct {
	Title       string
	Description string
	Hooks       []domain.HookCommand
}

// NewHookList creates a HookListModel seeded with the given hooks.
func NewHookList(params NewHookListParams) HookListModel {
	return HookListModel{
		hooks: params.Hooks,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
}

// Done returns true once the user selects the Done row.
func (m HookListModel) Done() bool { return m.done }

// Aborted returns true when the user presses Esc in browse mode.
func (m HookListModel) Aborted() bool { return m.aborted }

// Hooks returns the edited hook list.
func (m HookListModel) Hooks() []domain.HookCommand { return m.hooks }

// Init satisfies tea.Model.
func (m HookListModel) Init() tea.Cmd { return nil }

// addRow / doneRow are the cursor indices of the two trailing action rows.
func (m HookListModel) addRow() int  { return len(m.hooks) }
func (m HookListModel) doneRow() int { return len(m.hooks) + 1 }

// Update handles key events for browse and edit modes.
func (m HookListModel) Update(msg tea.Msg) (HookListModel, tea.Cmd) {
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
	case "shift+up", "K":
		if m.cursor > 0 && m.cursor < m.addRow() {
			m.hooks[m.cursor], m.hooks[m.cursor-1] = m.hooks[m.cursor-1], m.hooks[m.cursor]
			m.cursor--
		}
	case "shift+down", "J":
		if m.cursor < m.addRow()-1 {
			m.hooks[m.cursor], m.hooks[m.cursor+1] = m.hooks[m.cursor+1], m.hooks[m.cursor]
			m.cursor++
		}
	case "d":
		if m.cursor < m.addRow() {
			m.hooks = append(m.hooks[:m.cursor], m.hooks[m.cursor+1:]...)
		}
	case "enter":
		switch m.cursor {
		case m.doneRow():
			m.done = true
		case m.addRow():
			m = m.startEdit(-1)
		default:
			m = m.startEdit(m.cursor)
		}
	case "esc":
		m.aborted = true
	}

	return m, nil
}

// startEdit switches to the inline form for the hook at idx, or a new hook (-1).
func (m HookListModel) startEdit(idx int) HookListModel {
	cmd := textinput.New()
	cmd.CharLimit = hookInputCharLimit
	cmd.Width = max(hookInputMinWidth, m.width-hookInputWidthInset)
	cmd.Placeholder = "pnpm install"
	cwd := textinput.New()
	cwd.CharLimit = hookInputCharLimit
	cwd.Width = max(hookInputMinWidth, m.width-hookInputWidthInset)
	cwd.Placeholder = "repo root"

	if idx >= 0 {
		cmd.SetValue(m.hooks[idx].Cmd)
		cwd.SetValue(m.hooks[idx].Cwd)
		m.contOnErr = m.hooks[idx].ContinueOnError
	} else {
		m.contOnErr = false
	}

	m.editing = true
	m.editIndex = idx
	m.formFocus = 0
	m.formErr = ""
	m.cmdInput = cmd
	m.cwdInput = cwd
	return m.syncFocus()
}

func (m HookListModel) updateEdit(msg tea.Msg) (HookListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m.routeFormInput(msg)
	}

	switch keyMsg.String() {
	case "esc":
		m.editing = false
		return m, nil
	case "tab", "down":
		m.formFocus = (m.formFocus + 1) % hookFormFieldCount
		return m.syncFocus(), nil
	case "shift+tab", "up":
		m.formFocus = (m.formFocus + hookFormFieldCount - 1) % hookFormFieldCount
		return m.syncFocus(), nil
	case "enter":
		return m.saveEdit()
	case " ":
		if m.formFocus == 2 {
			m.contOnErr = !m.contOnErr
			return m, nil
		}
	}

	return m.routeFormInput(msg)
}

func (m HookListModel) routeFormInput(msg tea.Msg) (HookListModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.formFocus {
	case 0:
		m.cmdInput, cmd = m.cmdInput.Update(msg)
	case 1:
		m.cwdInput, cmd = m.cwdInput.Update(msg)
	}
	return m, cmd
}

func (m HookListModel) syncFocus() HookListModel {
	m.cmdInput.Blur()
	m.cwdInput.Blur()
	switch m.formFocus {
	case 0:
		m.cmdInput.Focus()
	case 1:
		m.cwdInput.Focus()
	}
	return m
}

func (m HookListModel) saveEdit() (HookListModel, tea.Cmd) {
	cmd := strings.TrimSpace(m.cmdInput.Value())
	if cmd == "" {
		m.formErr = "Command cannot be empty"
		return m, nil
	}

	hook := domain.HookCommand{
		Cmd:             cmd,
		Cwd:             strings.TrimSpace(m.cwdInput.Value()),
		ContinueOnError: m.contOnErr,
	}

	if m.editIndex >= 0 {
		m.hooks[m.editIndex] = hook
	} else {
		m.hooks = append(m.hooks, hook)
		m.cursor = len(m.hooks) - 1
	}

	m.editing = false
	return m, nil
}

// View renders the browse list or the edit form.
func (m HookListModel) View() string {
	if m.editing {
		return m.viewEdit()
	}

	var b strings.Builder
	for i, h := range m.hooks {
		m.renderRow(&b, fmt.Sprintf("%d. %s", i+1, hookLabel(h)), i == m.cursor)
		b.WriteString("\n")
	}
	m.renderRow(&b, "+ Add a command", m.cursor == m.addRow())
	b.WriteString("\n")
	m.renderRow(&b, "✓ Done", m.cursor == m.doneRow())
	return b.String()
}

func (m HookListModel) renderRow(b *strings.Builder, label string, selected bool) {
	if selected {
		line := "▸ " + label
		pad := m.width - PrintableWidth(line)
		if pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(styles.ListItemSelected.Render(line))
		return
	}
	b.WriteString(styles.ListItemNormal.Render(styles.Indent + label))
}

func (m HookListModel) viewEdit() string {
	var b strings.Builder
	b.WriteString(formField("Command", m.cmdInput.View(), m.formFocus == 0))
	b.WriteString("\n")
	b.WriteString(formField("Working dir (optional)", m.cwdInput.View(), m.formFocus == 1))
	b.WriteString("\n")
	check := "[ ]"
	if m.contOnErr {
		check = "[✓]"
	}
	b.WriteString(formField("Continue on error", check, m.formFocus == 2))
	if m.formErr != "" {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.formErr))
	}
	return b.String()
}

func formField(label, value string, focused bool) string {
	marker := "  "
	if focused {
		marker = styles.ListCursor.Render("▸ ")
	}
	return marker + styles.Bold.Render(label+": ") + value
}

// hookLabel renders a hook as a single readable line.
func hookLabel(h domain.HookCommand) string {
	label := h.Cmd
	var meta []string
	if h.Cwd != "" {
		meta = append(meta, "cwd: "+h.Cwd)
	}
	if h.ContinueOnError {
		meta = append(meta, "continue-on-error")
	}
	if len(meta) > 0 {
		label += "  (" + strings.Join(meta, ", ") + ")"
	}
	return label
}

// helpHint returns the mode-appropriate help bar text.
func (m HookListModel) helpActions() []string {
	return []string{domain.HelpDelete, domain.HelpReorder}
}

func (m HookListModel) helpModal() string {
	if m.editing {
		return domain.HookListEditHelp
	}
	return ""
}
