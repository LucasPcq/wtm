package runview

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// key answers a key: the job's while it has the keyboard, the filter's while
// it is typed, else the view's.
func (m *model) key(msg tea.KeyMsg) tea.Cmd {
	if m.focused {
		return m.focusKey(msg)
	}
	if m.filtering {
		return m.filterKey(msg)
	}

	m.said = ""
	switch msg.String() {
	case "q":
		// Leaving under a restart would cut it between its stop and its start: q waits, Ctrl+C does not.
		if m.acting != "" {
			m.said = fmt.Sprintf(underWayFmt, m.acting)
			return nil
		}
		return tea.Quit
	case "ctrl+c":
		return m.leave()
	case "enter":
		return m.focus()
	case "esc":
		switch {
		case m.reaching:
			m.reaching = false
		case m.filter != "":
			m.filter = ""
			return m.selectKey(m.resolve())
		default:
			return m.dismiss()
		}
	case "a":
		m.reaching = !m.reaching
	case "/":
		m.filtering = true
	case "s":
		return m.act(stopping)
	case "r":
		return m.act(restarting)
	case "ctrl+r":
		return m.list()
	case "up", "k":
		return m.move(-1)
	case "down", "j":
		return m.move(1)
	case "pgup":
		m.scroll(m.layout().paneRows)
	case "pgdown":
		m.scroll(-m.layout().paneRows)
	case "shift+up":
		m.scroll(scrollLines)
	case "shift+down":
		m.scroll(-scrollLines)
	case "G":
		if m.pane != nil {
			m.pane.live()
		}
	case "o":
		return m.open(m.selectedURL())
	}
	return nil
}

// An action is what s and r do to the job selected.
type action int

const (
	stopping action = iota
	restarting
)

// act stops the job selected, or starts it again, through the view's
// params, one at a time: a shared service is released, never restarted for
// one worktree. What became of it is a notice, and the jobs are listed again.
func (m *model) act(a action) tea.Cmd {
	e, found := m.selectedEntry()
	if !found {
		return nil
	}
	do, verb, doing, done := m.params.Stop, "stop", "stopping", "stopped"
	if e.shared {
		verb, doing, done = "release", "releasing", "released"
	}
	if a == restarting {
		do, verb, doing, done = m.params.Restart, "restart", "restarting", "restarted"
	}
	switch {
	case do == nil:
		m.said = fmt.Sprintf(notActedFmt, e.name, verb)
		return nil
	case m.acting != "":
		m.said = fmt.Sprintf(underWayFmt, m.acting)
		return nil
	case a == restarting && e.shared:
		m.said = fmt.Sprintf(restartSharedFmt, e.name)
		return nil
	}

	act := Act{Worktree: e.branch, Dir: e.dir, Job: e.name}
	again, check := "", ""
	if m.params.Again != nil {
		again = m.params.Again(act)
	}
	// A stop left under way is checked with a read: starting the job again is what its user did not want.
	check = again
	if a == stopping {
		check = ""
		if m.params.Show != nil {
			check = m.params.Show(act)
		}
	}
	m.acting = fmt.Sprintf(actingFmt, doing, e.name)
	m.unended = fmt.Sprintf(unendedFmt, m.acting, check)
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancelAct = cancel
	return func() tea.Msg {
		defer cancel()
		acted, err := do(ctx, act)
		switch {
		// Cut short before it did anything: nothing to say once the view is left.
		case errors.Is(err, context.Canceled):
			return actedMsg{text: fmt.Sprintf(actFailedFmt, verb, e.name, err)}
		case err != nil:
			failed := fmt.Sprintf(actFailedFmt, verb, e.name, err)
			return actedMsg{text: failed, left: failed}
		case acted.CutShort:
			cut := fmt.Sprintf(cutShortFmt, e.name, e.name, again)
			return actedMsg{text: cut, left: cut}
		case acted.Unchanged:
			return actedMsg{text: fmt.Sprintf(notActedFmt, e.name, verb)}
		}
		return actedMsg{text: fmt.Sprintf(actedFmt, e.name, done)}
	}
}

// acted takes in the end of an action: said as a notice, the jobs listed again; the view left once Ctrl+C asked it, a restart cut short said after it.
func (m *model) acted(msg actedMsg) tea.Cmd {
	m.acting, m.cancelAct = "", nil
	if m.leaving {
		m.left = msg.left
		return tea.Quit
	}
	m.say(msg.text)
	return tea.Batch(m.resize(), m.list())
}

// actWait bounds how long Ctrl+C waits for the action it cut short to end:
// a stop sent is let finish.
const actWait = 30 * time.Second

// leave leaves the view; with an action under way, it cuts the action short
// first, and leaves once it ended, or once actWait passed or Ctrl+C came again,
// saying then that it still runs.
func (m *model) leave() tea.Cmd {
	switch {
	case m.acting == "":
		return tea.Quit
	// A second Ctrl+C leaves at once, saying the action still runs.
	case m.leaving:
		m.left = m.unended
		return tea.Quit
	}
	m.leaving = true
	m.said = fmt.Sprintf(leavingFmt, m.acting)
	m.cancelAct()
	return tea.Tick(actWait, func(time.Time) tea.Msg { return leaveMsg{} })
}

// focus hands the keyboard to the job selected, when it is attached and reads
// a terminal of its own.
func (m *model) focus() tea.Cmd {
	e, found := m.selectedEntry()
	if !found {
		return nil
	}
	if refused := m.refusal(e); refused != "" {
		m.said = refused
		return nil
	}
	if m.conn == nil {
		m.said = fmt.Sprintf(noStreamFmt, e.name)
		return nil
	}
	m.focused, m.lastEsc = true, time.Time{}
	m.pane.live()
	return nil
}

// focusKey types the key into the job, Ctrl+C included; esc twice within the
// window gives the keyboard back, a lone esc being the job's.
func (m *model) focusKey(msg tea.KeyMsg) tea.Cmd {
	now := m.now()
	if msg.Type == tea.KeyEsc && !msg.Alt {
		if !m.lastEsc.IsZero() && now.Sub(m.lastEsc) <= focusExitWindow {
			m.focused, m.lastEsc = false, time.Time{}
			return nil
		}
		m.lastEsc = now
	} else {
		m.lastEsc = time.Time{}
	}

	// Written here, on the loop, so the keys reach the job in the order typed;
	// the daemon reads what a client types at once.
	encoded := encode(msg)
	if m.conn == nil || len(encoded) == 0 {
		return nil
	}
	if _, err := m.conn.Write(encoded); err != nil {
		m.focused = false
		m.say(fmt.Sprintf(typeFailedFmt, m.label(m.bound.key), err))
		return m.resize()
	}
	return nil
}

// filterKey types into the filter; the cursor moves off a job the filter hides.
func (m *model) filterKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		m.filtering, m.filter = false, ""
	case "enter":
		m.filtering = false
		return nil
	case "backspace":
		runes := []rune(m.filter)
		if len(runes) == 0 {
			return nil
		}
		m.filter = string(runes[:len(runes)-1])
	default:
		if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
			return nil
		}
		m.filter += string(msg.Runes)
	}
	return m.selectKey(m.resolve())
}

// dismiss takes the band off the frame, its rows given back to the pane.
func (m *model) dismiss() tea.Cmd {
	if len(m.band()) == 0 {
		return nil
	}
	m.dismissed = true
	return m.resize()
}

func (m *model) move(delta int) tea.Cmd {
	list := m.visible()
	if len(list) == 0 {
		return nil
	}
	at := 0
	for i, e := range list {
		if e.key == m.selected {
			at = i
		}
	}
	return m.selectKey(list[min(max(at+delta, 0), len(list)-1)].key)
}

func (m *model) scroll(lines int) {
	if m.pane != nil {
		m.pane.scroll(lines)
	}
}

// ----------------------------------------------------------------------------
// The mouse

// mouse sends the wheel to what it is over, the list or the pane, and opens
// the address a click lands on; the job with the keyboard keeps the mouse off.
func (m *model) mouse(msg tea.MouseMsg) tea.Cmd {
	if m.focused || m.filtering {
		return nil
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
		url, found := urlAt(m.View(), msg.X, msg.Y)
		if !found {
			return nil
		}
		return m.open(url)
	}

	up := msg.Button == tea.MouseButtonWheelUp
	if !up && msg.Button != tea.MouseButtonWheelDown {
		return nil
	}
	m.said = ""
	if l := m.layout(); l.sidebar.width > 0 && msg.X >= l.sidebar.x && msg.X < l.sidebar.x+l.sidebar.width {
		if up {
			return m.move(-1)
		}
		return m.move(1)
	}
	if up {
		m.scroll(scrollLines)
	} else {
		m.scroll(-scrollLines)
	}
	return nil
}

var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

// urlAt is the address drawn under the cell at x, y of view.
func urlAt(view string, x, y int) (string, bool) {
	lines := strings.Split(view, "\n")
	if y < 0 || y >= len(lines) {
		return "", false
	}
	plain := ansi.Strip(lines[y])
	for _, span := range urlPattern.FindAllStringIndex(plain, -1) {
		start := ansi.StringWidth(plain[:span[0]])
		if x >= start && x < start+ansi.StringWidth(plain[span[0]:span[1]]) {
			return plain[span[0]:span[1]], true
		}
	}
	return "", false
}

// selectedURL is the address of the job selected, empty when it has none.
func (m *model) selectedURL() string {
	e, found := m.selectedEntry()
	if !found {
		return ""
	}
	return e.job.URL
}

// open opens url in the browser; a job with none opens nothing, and a browser
// that refuses is said.
func (m *model) open(url string) tea.Cmd {
	if url == "" || m.params.Open == nil {
		return nil
	}
	open := m.params.Open
	return func() tea.Msg {
		if err := open(url); err != nil {
			return saidMsg{text: fmt.Sprintf(openFailedFmt, err)}
		}
		return nil
	}
}

// ----------------------------------------------------------------------------
// A key typed into a job

// sequences are what a terminal sends for the keys that are no text; ctrl+a
// to ctrl+z are computed.
var sequences = map[string]string{
	"enter": "\r", "tab": "\t", "shift+tab": "\x1b[Z", "esc": "\x1b", "backspace": "\x7f",
	"delete": "\x1b[3~", "insert": "\x1b[2~", " ": " ",
	"up": "\x1b[A", "down": "\x1b[B", "right": "\x1b[C", "left": "\x1b[D", "home": "\x1b[H", "end": "\x1b[F",
	"pgup": "\x1b[5~", "pgdown": "\x1b[6~",
	"ctrl+@": "\x00", "ctrl+\\": "\x1c", "ctrl+]": "\x1d", "ctrl+^": "\x1e", "ctrl+_": "\x1f",
	"f1": "\x1bOP", "f2": "\x1bOQ", "f3": "\x1bOR", "f4": "\x1bOS", "f5": "\x1b[15~", "f6": "\x1b[17~",
	"f7": "\x1b[18~", "f8": "\x1b[19~", "f9": "\x1b[20~", "f10": "\x1b[21~", "f11": "\x1b[23~", "f12": "\x1b[24~",
	"shift+up": "\x1b[1;2A", "shift+down": "\x1b[1;2B", "shift+right": "\x1b[1;2C", "shift+left": "\x1b[1;2D",
	"shift+home": "\x1b[1;2H", "shift+end": "\x1b[1;2F",
	"ctrl+up": "\x1b[1;5A", "ctrl+down": "\x1b[1;5B", "ctrl+right": "\x1b[1;5C", "ctrl+left": "\x1b[1;5D",
	"ctrl+home": "\x1b[1;5H", "ctrl+end": "\x1b[1;5F", "ctrl+pgup": "\x1b[5;5~", "ctrl+pgdown": "\x1b[6;5~",
	"ctrl+shift+up": "\x1b[1;6A", "ctrl+shift+down": "\x1b[1;6B", "ctrl+shift+right": "\x1b[1;6C",
	"ctrl+shift+left": "\x1b[1;6D", "ctrl+shift+home": "\x1b[1;6H", "ctrl+shift+end": "\x1b[1;6F",
}

// encode is the bytes a terminal sends for msg, none for a key it has no
// encoding of: a stray byte in a job's input is worse than a key lost.
func encode(msg tea.KeyMsg) []byte {
	var encoded []byte
	name := msg.Type.String()
	switch sequence, named := sequences[name]; {
	case msg.Type == tea.KeyRunes:
		encoded = []byte(string(msg.Runes))
	case named:
		encoded = []byte(sequence)
	case strings.HasPrefix(name, "ctrl+") && len(name) == len("ctrl+a") && name[5] >= 'a' && name[5] <= 'z':
		encoded = []byte{name[5] - 'a' + 1}
	}
	if len(encoded) > 0 && msg.Alt {
		return append([]byte{0x1b}, encoded...)
	}
	return encoded
}
