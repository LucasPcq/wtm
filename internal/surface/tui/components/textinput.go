package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// TextInputModel is a prompt-style text field with validation.
type TextInputModel struct {
	input    textinput.Model
	title    string
	desc     string
	validate func(string) error
	width    int
	done     bool
	aborted  bool
	err      error
}

// NewTextInput creates a TextInputModel with a title and description.
func NewTextInput(params NewTextInputParams) TextInputModel {
	ti := newPromptInput()

	if params.Placeholder != "" {
		ti.Placeholder = params.Placeholder
	}
	if params.Default != "" {
		ti.SetValue(params.Default)
	}

	m := TextInputModel{
		input:    ti,
		title:    params.Title,
		desc:     params.Description,
		validate: params.Validate,
		width:    80,
	}

	return m
}

func newPromptInput() textinput.Model {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = styles.InputPrompt.Render("❯ ")
	ti.Width = 76
	return ti
}

// NewTextInputParams holds inputs for NewTextInput.
type NewTextInputParams struct {
	Title       string
	Description string
	Placeholder string
	Default     string
	Validate    func(string) error
}

// Done returns true after the user confirmed their input.
func (m TextInputModel) Done() bool { return m.done }

// Aborted returns true after the user pressed Esc.
func (m TextInputModel) Aborted() bool { return m.aborted }

// Value returns the current input text.
func (m TextInputModel) Value() string { return strings.TrimSpace(m.input.Value()) }

// Placeholder returns the placeholder text.
func (m TextInputModel) Placeholder() string { return m.input.Placeholder }

// SetWidth updates the available width.
func (m *TextInputModel) SetWidth(w int) {
	m.width = w
	m.input.Width = max(10, w-4)
}

// Init starts the cursor blink.
func (m TextInputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles key events.
func (m TextInputModel) Update(msg tea.Msg) (TextInputModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if m.validate != nil {
				if err := m.validate(m.input.Value()); err != nil {
					m.err = err
					return m, nil
				}
			}
			m.done = true
			return m, nil
		case "esc":
			m.aborted = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.validate != nil {
		if err := m.validate(m.input.Value()); err != nil {
			m.err = err
		} else {
			m.err = nil
		}
	} else {
		m.err = nil
	}
	return m, cmd
}

// View renders the prompt-style input.
// A text input has no rows, so the bar must not offer to move between them.
func (m TextInputModel) helpActions() []string { return nil }

func (m TextInputModel) helpModal() string { return "" }

// helpRowless answers the `rowless` interface of the wizard's help bar: a text
// input has no rows, so it must not offer to move between them.
func (m TextInputModel) helpRowless() bool { return true }

func (m TextInputModel) View() string {
	var b strings.Builder

	b.WriteString(m.input.View())

	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(wrappedErrorBanner(m.err.Error(), m.width))
	}

	return b.String()
}
