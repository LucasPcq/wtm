// Package formtest tests a command's form: a driver that plays its wizard key by key, golden files, and the v2 envelope decoded.
package formtest

import (
	"fmt"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const settleWithin = 100 * time.Millisecond

// A Driver plays a wizard's event loop in the test's goroutine, the way Bubble Tea would: it sends a message, runs the command it returns, and feeds back the message that command produces.
type Driver struct {
	Wizard *form.Wizard
}

// Start sizes w's terminal and runs its first command.
func Start(w *form.Wizard, size tea.WindowSizeMsg) *Driver {
	d := &Driver{Wizard: w}
	d.Send(size)
	d.Run(w.Init())
	return d
}

// Send hands msg to the wizard and runs the command it returns.
func (d *Driver) Send(msg tea.Msg) {
	_, cmd := d.Wizard.Update(msg)
	d.Run(cmd)
}

// Run runs cmd and sends back its message, a batch's one by one; a command still running after settleWithin, a tick or a cursor blink is dropped, so the loop settles.
func (d *Driver) Run(cmd tea.Cmd) {
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
			d.Run(inner)
		}
	default:
		if !strings.Contains(fmt.Sprintf("%T", m), "blink") {
			d.Send(m)
		}
	}
}

// Keys presses each key in turn, named as [Key] names them.
func (d *Driver) Keys(keys ...string) {
	for _, key := range keys {
		d.Send(Key(key))
	}
}

// Screen is the wizard's view as plain text.
func (d *Driver) Screen() string {
	return Plain(d.Wizard.View())
}

// Key is the key press name stands for: enter, esc, tab, up, down, space, backspace or ctrl+c, else the runes of name typed.
func Key(name string) tea.KeyMsg {
	switch name {
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
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// Plain is view without its escape codes, each line's trailing spaces and the trailing blank lines.
func Plain(view string) string {
	lines := strings.Split(ansi.Strip(view), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
