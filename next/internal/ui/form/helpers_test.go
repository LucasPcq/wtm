package form

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const settleWithin = 100 * time.Millisecond

// A driver plays a wizard's event loop in the test's goroutine, the way Bubble Tea would.
type driver struct {
	w *Wizard
}

func open(t *testing.T, params Params) *driver {
	t.Helper()
	return start(New(t.Context(), params))
}

func start(w *Wizard) *driver {
	d := &driver{w: w}
	d.send(tea.WindowSizeMsg{Width: 80, Height: 30})
	d.run(w.Init())
	return d
}

func (d *driver) send(msg tea.Msg) {
	_, cmd := d.w.Update(msg)
	d.run(cmd)
}

// run runs cmd and sends the wizard the message it returns, a batch's one by one; a command that does not return within settleWithin, such as a tick, is dropped.
func (d *driver) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-result:
	case <-time.After(settleWithin):
		return
	}
	switch m := msg.(type) {
	case nil, tea.QuitMsg, spinner.TickMsg:
	case tea.BatchMsg:
		for _, inner := range m {
			d.run(inner)
		}
	default:
		d.send(m)
	}
}

func (d *driver) keys(keys ...string) {
	for _, key := range keys {
		d.send(keyOf(key))
	}
}

func keyOf(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func (d *driver) screen() string {
	return plain(d.w.View())
}

func plain(view string) string {
	lines := strings.Split(ansi.Strip(view), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// settled runs the checks cmd starts on l, and those their verdicts start in turn, and returns the verdict they end on.
func settled(l *TextList, cmd tea.Cmd) Verdict {
	verdict := Stay
	for _, msg := range messages(cmd) {
		if checked, ok := msg.(entryChecked); ok {
			var next tea.Cmd
			verdict, next = l.Update(checked)
			if verdict == Stay {
				verdict = settled(l, next)
			}
		}
	}
	return verdict
}

// textSettled runs the check cmd starts on x and returns the verdict it ends on.
func textSettled(x *Text, cmd tea.Cmd) Verdict {
	verdict := Stay
	for _, msg := range messages(cmd) {
		if checked, ok := msg.(textChecked); ok {
			verdict, _ = x.Update(checked)
		}
	}
	return verdict
}

// messages returns what cmd returns, a batch flattened; nil when it does not return within settleWithin.
func messages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	select {
	case msg := <-result:
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			return []tea.Msg{msg}
		}
		var all []tea.Msg
		for _, inner := range batch {
			all = append(all, messages(inner)...)
		}
		return all
	case <-time.After(settleWithin):
		return nil
	}
}

func plainStyles() *Styles {
	return newStyles(output.StylesOf(io.Discard))
}

func pressed(m *MultiSelect, keys ...string) Verdict {
	verdict := Stay
	for _, key := range keys {
		verdict, _ = m.Update(keyOf(key))
	}
	return verdict
}
