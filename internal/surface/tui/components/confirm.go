package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// ConfirmModel is a yes/no prompt with full-row highlight.
type ConfirmModel struct {
	title     string
	desc      string
	warning   string
	cursor    int
	width     int
	done      bool
	aborted   bool
	confirmed bool
}

// NewConfirm creates a ConfirmModel.
func NewConfirm(params NewConfirmParams) ConfirmModel {
	cursor := 1
	if params.DefaultYes {
		cursor = 0
	}
	return ConfirmModel{
		title:   params.Title,
		desc:    params.Description,
		warning: params.Warning,
		width:   80,
		cursor:  cursor,
	}
}

// NewConfirmParams holds inputs for NewConfirm.
type NewConfirmParams struct {
	Title       string
	Description string
	// Warning, when non-empty, renders above the Yes/No options as a styled
	// "! message" banner — same look as render.Warning, mirrored inside the wizard.
	Warning    string
	DefaultYes bool
}

// Done returns true after the user confirmed or denied.
func (m ConfirmModel) Done() bool { return m.done }

// Aborted returns true after the user pressed Esc.
func (m ConfirmModel) Aborted() bool { return m.aborted }

// Confirmed returns true if the user chose Yes.
func (m ConfirmModel) Confirmed() bool { return m.confirmed }

// Init satisfies tea.Model.
func (m ConfirmModel) Init() tea.Cmd { return nil }

// Update handles key events.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
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
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter":
		m.done = true
		m.confirmed = m.cursor == 0
	case "esc", "ctrl+c":
		m.aborted = true
	}

	return m, nil
}

// View renders the yes/no prompt.
func (m ConfirmModel) View() string {
	var b strings.Builder

	if m.warning != "" {
		b.WriteString(warningBanner(m.warning))
		b.WriteString("\n\n")
	}

	options := [2]string{"Yes", "No"}

	for i, opt := range options {
		selected := i == m.cursor

		if selected {
			line := "▸ " + opt
			pad := m.width - len(line)
			if pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			b.WriteString(styles.ListItemSelected.Render(line))
		} else {
			b.WriteString(styles.ListItemNormal.Render(styles.Indent + opt))
		}

		if i == 0 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
