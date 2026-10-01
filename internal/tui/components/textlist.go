package components

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

type TextListEntry struct {
	Entry   string
	Entries []string
}

type NewTextListParams struct {
	Title       string
	Description string
	Entries     []string
	Check       func(TextListEntry) (string, error)
	Badge       func(entry string) Badge
	// Required is the refusal shown when enter would continue with no entry.
	Required string
}

type TextListModel struct {
	input    textinput.Model
	entries  []string
	title    string
	desc     string
	check    func(TextListEntry) (string, error)
	badge    func(string) Badge
	required string
	width    int
	done     bool
	aborted  bool
	err      error
}

func NewTextList(params NewTextListParams) TextListModel {
	return TextListModel{
		input:    newPromptInput(),
		entries:  append([]string(nil), params.Entries...),
		title:    params.Title,
		desc:     params.Description,
		check:    params.Check,
		badge:    params.Badge,
		required: params.Required,
		width:    80,
	}
}

func (m TextListModel) Done() bool { return m.done }

func (m TextListModel) Aborted() bool { return m.aborted }

func (m TextListModel) Values() []string { return append([]string(nil), m.entries...) }

func (m TextListModel) Init() tea.Cmd { return textinput.Blink }

func (m TextListModel) Update(msg tea.Msg) (TextListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// The cursor blinks through here too: only a keystroke dismisses a refusal.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	switch keyMsg.String() {
	case "tab":
		m.addTyped()
		return m, nil
	case "enter":
		return m.confirm(), nil
	case "esc":
		m.aborted = true
		return m, nil
	case "backspace":
		if m.input.Value() == "" && len(m.entries) > 0 {
			m.entries = m.entries[:len(m.entries)-1]
			m.err = nil
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = nil
	return m, cmd
}

// A refused entry stays in the field, with its reason under it.
func (m *TextListModel) addTyped() (cleared bool) {
	typed := m.input.Value()
	if strings.TrimSpace(typed) == "" {
		return true
	}
	entry := strings.TrimSpace(typed)
	if m.check != nil {
		checked, err := m.check(TextListEntry{Entry: typed, Entries: m.entries})
		if err != nil {
			m.err = err
			return false
		}
		entry = checked
	}
	m.entries = append(m.entries, entry)
	m.input.SetValue("")
	m.err = nil
	return true
}

func (m TextListModel) confirm() TextListModel {
	if !m.addTyped() {
		return m
	}
	if len(m.entries) == 0 {
		m.err = errors.New(m.required)
		return m
	}
	m.done = true
	return m
}

func (m TextListModel) helpActions() []string {
	return []string{domain.HelpAddAnother, domain.HelpRemoveLast}
}

func (m TextListModel) helpModal() string { return "" }

func (m TextListModel) helpRowless() bool { return true }

func (m TextListModel) View() string {
	var b strings.Builder
	b.WriteString(m.input.View())
	for _, entry := range m.entries {
		b.WriteString("\n" + styles.Indent + entry)
		if m.badge == nil {
			continue
		}
		if badge := m.badge(entry); badge.Text != "" {
			b.WriteString("  " + badge.Render())
		}
	}
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.err.Error()))
	}
	return b.String()
}
