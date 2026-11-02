package runview

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const settleWithin = 100 * time.Millisecond

// A driver plays the view's event loop in the test's goroutine: it sends a
// message, runs the command it returns, and feeds back what that produces; a
// command still waiting after settleWithin (the poll, the frame clock, a
// stream's end) is dropped, so the loop settles.
type driver struct {
	t      *testing.T
	m      *Model
	daemon *daemon
	quit   bool
}

// open opens the view of params over d at width × height, its first listing read.
func open(t *testing.T, d *daemon, params Params, width, height int) *driver {
	t.Helper()
	params.Daemon = d
	params.Out = io.Discard
	m := New(t.Context(), params)
	m.now = func() time.Time { return now }
	dr := &driver{t: t, m: m, daemon: d}
	dr.send(tea.WindowSizeMsg{Width: width, Height: height})
	dr.run(m.Init())
	return dr
}

// opened is the board's view, the jobs listed as board says, each job attached
// to a terminal unless d attaches otherwise.
func opened(t *testing.T, d *daemon, width, height int) *driver {
	t.Helper()
	worktrees, jobs := board()
	d.set(jobs)
	if d.attach == nil {
		piped(d)
	}
	return open(t, d, Params{Worktrees: worktrees, Main: "main"}, width, height)
}

func (d *driver) send(msg tea.Msg) {
	_, cmd := d.m.Update(msg)
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
	case nil:
	case tea.QuitMsg:
		d.quit = true
	case tea.BatchMsg:
		for _, inner := range m {
			d.run(inner)
		}
	default:
		d.send(m)
	}
}

// keys presses each key in turn, named as formtest.Key names them, or a tea key name.
func (d *driver) keys(names ...string) {
	for _, name := range names {
		d.send(keyOf(name))
	}
}

func keyOf(name string) tea.KeyMsg {
	switch name {
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "shift+up":
		return tea.KeyMsg{Type: tea.KeyShiftUp}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	}
	return formtest.Key(name)
}

// refresh reads the daemon's jobs again, as the poll does.
func (d *driver) refresh() {
	d.run(d.m.list())
}

func (d *driver) screen() string {
	return formtest.Plain(d.m.View())
}

// helpRow is the last row of the frame.
func (d *driver) helpRow() string {
	lines := strings.Split(d.screen(), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// waitFor redraws until the screen holds text, or fails after a second.
func (d *driver) waitFor(text string) {
	d.t.Helper()
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(d.screen(), text) {
		if time.Now().After(deadline) {
			d.t.Fatalf("the screen never showed %q:\n%s", text, d.screen())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// locate is the row and the column at which text is drawn in view.
func locate(t *testing.T, view, text string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(view), "\n") {
		if x := strings.Index(line, text); x >= 0 {
			return y, ansi.StringWidth(line[:x])
		}
	}
	t.Fatalf("%q is not drawn:\n%s", text, view)
	return 0, 0
}
