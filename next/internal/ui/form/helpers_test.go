package form

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const settleWithin = 100 * time.Millisecond

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

func pressed(m *MultiSelect, keys ...string) Verdict {
	verdict := Stay
	for _, key := range keys {
		verdict, _ = m.Update(keyOf(key))
	}
	return verdict
}
