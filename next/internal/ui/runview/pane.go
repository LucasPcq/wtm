package runview

import (
	"bytes"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

const (
	scrollbackLines = 1000
	// scrollbackBurst widens the history while the reader scrolls back, so the
	// lines they read are not evicted under them by what the job prints.
	scrollbackBurst = 2
	defaultCols     = 80
	defaultRows     = 24
)

// A size is a pane's in columns and rows.
type size struct {
	cols int
	rows int
}

// A pane is a job's terminal drawn by an emulator: what it printed, its
// scrollback, how far back the reader scrolled. The reader of a job's stream
// writes into it while the view draws it, hence its lock: vt.Emulator guards
// none of its own.
type pane struct {
	mu     sync.Mutex
	term   *vt.Emulator
	size   size
	offset int
	wake   chan<- struct{}
	// crlf gives a pipe's bare line feeds the carriage return a terminal adds;
	// pendingCR is a chunk that ended on one, so a CRLF split across two writes
	// does not gain a second.
	crlf      bool
	pendingCR bool
}

// paneParams are a pane's size, whether its job prints through a pipe, and
// where it says it was written (nil: nowhere).
type paneParams struct {
	at   size
	crlf bool
	wake chan<- struct{}
}

func newPane(params paneParams) *pane {
	at := sized(params.at)
	p := &pane{term: vt.NewEmulator(at.cols, at.rows), size: at, crlf: params.crlf, wake: params.wake}
	p.term.SetScrollbackSize(scrollbackLines)
	return p
}

func sized(s size) size {
	if s.cols <= 0 {
		s.cols = defaultCols
	}
	if s.rows <= 0 {
		s.rows = defaultRows
	}
	return s
}

// Write feeds the emulator what the job printed.
func (p *pane) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(b)
	if p.crlf {
		b = p.terminated(b)
	}
	held := p.term.ScrollbackLen()
	_, err := p.term.Write(b)
	p.follow(held)
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return n, err
}

func (p *pane) terminated(chunk []byte) []byte {
	out := make([]byte, 0, len(chunk)+bytes.Count(chunk, []byte{'\n'}))
	for _, b := range chunk {
		if b == '\n' && !p.pendingCR {
			out = append(out, '\r')
		}
		out = append(out, b)
		p.pendingCR = b == '\r'
	}
	return out
}

// resize sizes the emulator again and reports whether its size changed. The
// emulator never reflows: the rows a shorter screen loses are scrolled into
// the history first, or the newest output would go.
func (p *pane) resize(to size) bool {
	to = sized(to)
	p.mu.Lock()
	defer p.mu.Unlock()
	if to == p.size {
		return false
	}
	if lost := p.term.CursorPosition().Y - (to.rows - 1); lost > 0 {
		held := p.term.ScrollbackLen()
		_, _ = p.term.Write([]byte(ansi.ScrollUp(lost)))
		p.follow(held)
	}
	p.term.Resize(to.cols, to.rows)
	p.size = to
	p.clamp()
	return true
}

// scroll moves the reader lines back into the history, forward when negative;
// zero is the live end.
func (p *pane) scroll(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset += lines
	p.clamp()
	p.holdHistory()
}

func (p *pane) live() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset = 0
	p.holdHistory()
}

func (p *pane) scrolled() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.offset
}

// render is the rows on screen, with their colours.
func (p *pane) render() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.offset == 0 {
		return p.term.Render()
	}

	history := p.term.Scrollback()
	lines := make([]string, 0, p.size.rows)
	for i := p.offset; i > 0 && len(lines) < p.size.rows; i-- {
		lines = append(lines, ansi.Truncate(history.Line(history.Len()-i).Render(), p.size.cols, ""))
	}
	for line := range strings.SplitSeq(p.term.Render(), "\n") {
		if len(lines) == p.size.rows {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// follow keeps a reader scrolled back on the same lines as a write pushes the
// history up; a history full already slides, and the reader then holds its
// oldest line.
func (p *pane) follow(held int) {
	if p.offset == 0 {
		return
	}
	if p.term.ScrollbackLen() >= scrollbackLines*scrollbackBurst {
		p.offset = p.term.ScrollbackLen()
		return
	}
	p.offset += p.term.ScrollbackLen() - held
	p.clamp()
}

func (p *pane) holdHistory() {
	if p.offset > 0 {
		p.term.SetScrollbackSize(scrollbackLines * scrollbackBurst)
		return
	}
	p.term.SetScrollbackSize(scrollbackLines)
}

func (p *pane) clamp() {
	p.offset = min(max(p.offset, 0), p.term.ScrollbackLen())
}
