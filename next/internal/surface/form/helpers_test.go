package form

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const settleWithin = 100 * time.Millisecond

type driver struct {
	t *testing.T
	w *wizard
}

func open(t *testing.T, params wizardParams) *driver {
	t.Helper()
	return start(t, newWizard(t.Context(), params))
}

func start(t *testing.T, w *wizard) *driver {
	t.Helper()
	d := &driver{t: t, w: w}
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
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
		if !strings.Contains(fmt.Sprintf("%T", m), "blink") {
			d.send(m)
		}
	}
}

func (d *driver) keys(keys ...string) {
	for _, key := range keys {
		switch key {
		case "enter":
			d.send(tea.KeyMsg{Type: tea.KeyEnter})
		case "tab":
			d.send(tea.KeyMsg{Type: tea.KeyTab})
		case "esc":
			d.send(tea.KeyMsg{Type: tea.KeyEsc})
		case "up":
			d.send(tea.KeyMsg{Type: tea.KeyUp})
		case "down":
			d.send(tea.KeyMsg{Type: tea.KeyDown})
		case "backspace":
			d.send(tea.KeyMsg{Type: tea.KeyBackspace})
		case "ctrl+c":
			d.send(tea.KeyMsg{Type: tea.KeyCtrlC})
		default:
			d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		}
	}
}

func (d *driver) screen() string {
	lines := strings.Split(ansi.Strip(d.w.View()), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func engineScreen(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "engine_"+name+".txt"))
	require.NoError(t, err)
	return strings.TrimRight(engineGlyphsFontCovered.Replace(string(data)), "\n")
}

var engineGlyphsFontCovered = strings.NewReplacer("▸", "›", "❯", "›", "⊘", "·")

func (d *driver) looksLike(name string) {
	d.t.Helper()
	assert.Equal(d.t, engineScreen(d.t, name), d.screen())
}
