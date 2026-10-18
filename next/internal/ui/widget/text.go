package widget

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// A Text is a [Widget] that takes one typed value: enter checks it off the event loop and answers the field once it is accepted; keys typed meanwhile wait for the check.
type Text struct {
	input    textinput.Model
	check    func(value string) error
	err      error
	checking bool
	queued   []tea.KeyMsg
}

// A textChecked brings a check's verdict back to the event loop.
type textChecked struct {
	err error
}

// NewText returns a text field holding value; check accepts the value typed or returns the error to show.
func NewText(value string, check func(value string) error) *Text {
	input := textinput.New()
	input.SetValue(value)
	input.Focus()
	return &Text{input: input, check: check}
}

// Value returns the value typed, trimmed.
func (t *Text) Value() string {
	return strings.TrimSpace(t.input.Value())
}

// Update handles a check that landed, a key, or a message of the text input, such as its cursor's blink.
func (t *Text) Update(msg tea.Msg) (Verdict, tea.Cmd) {
	if checked, ok := msg.(textChecked); ok {
		return t.checked(checked.err)
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		t.input, cmd = t.input.Update(msg)
		return Stay, cmd
	}

	if t.checking {
		t.queued = append(t.queued, key)
		return Stay, nil
	}

	switch key.String() {
	case "enter":
		t.checking = true
		check, value := t.check, t.Value()
		return Stay, func() tea.Msg { return textChecked{err: check(value)} }
	case "esc":
		return Back, nil
	}

	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	t.err = nil
	return Stay, cmd
}

// checked answers the field once the value is accepted, else shows the error and replays the keys that waited, until one ends the field or starts another check.
func (t *Text) checked(err error) (Verdict, tea.Cmd) {
	t.checking, t.err = false, err
	if err == nil {
		t.queued = nil
		return Done, nil
	}

	var cmds []tea.Cmd
	for len(t.queued) > 0 && !t.checking {
		key := t.queued[0]
		t.queued = t.queued[1:]
		verdict, cmd := t.Update(key)
		cmds = append(cmds, cmd)
		if verdict != Stay {
			t.queued = nil
			return verdict, tea.Batch(cmds...)
		}
	}
	return Stay, tea.Batch(cmds...)
}

// View draws the input, then the error of a refused value.
func (t *Text) View(size Size, st *Styles) string {
	t.input.Prompt = st.Accent.Render("› ")
	t.input.Width = max(10, size.Width-4)

	if t.err == nil {
		return t.input.View()
	}
	return t.input.View() + "\n\n" + errorBanner(t.err.Error(), size.Width, st)
}

// Help adds no key: the field has no rows to navigate.
func (t *Text) Help() Help {
	return Help{Rowless: true}
}
