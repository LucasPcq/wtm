package daemon

import (
	"bytes"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	// historyBytes is how much of a job's raw output a client attaching to its terminal is replayed: enough for a terminal to draw the screen again, escapes included.
	historyBytes = 1 << 20
	// tailBytes caps the text of the lines kept, as run.MaxTail caps their count: a job printing long lines keeps fewer.
	tailBytes   = 1 << 20
	followQueue = 1024
	inputQueue  = 64
)

// An output is what a job prints, as it comes: its last lines, cut and
// timestamped, written to its log, handed to the start's client while it
// waits (out) and to each follower; its last raw bytes, handed to each client
// attached to its terminal. A follower or a client attached that falls behind
// is let go. Its buffers are cut back to their caps once they doubled them,
// never on each write. The log and the start's client are written outside its
// lock, by the one goroutine that writes: a slow disk or client holds no
// reader of the listing. The start's client is let go at the output's end.
type output struct {
	now  func() time.Time
	log  Log
	last atomic.Pointer[run.Line]

	mu        sync.Mutex
	out       func(run.Line)
	splitter  run.Splitter
	lines     []run.Line
	text      int
	raw       []byte
	followers map[*follower]struct{}
	attached  map[chan []byte]struct{}
	ended     bool
}

// A follower is a client following a job's lines; behind once it fell behind
// and was let go, its channel closed as at the job's end.
type follower struct {
	lines  chan run.Line
	behind atomic.Bool
}

func newOutput(now func() time.Time) *output {
	return &output{now: now, followers: map[*follower]struct{}{}, attached: map[chan []byte]struct{}{}}
}

func (o *output) write(chunk []byte) {
	o.mu.Lock()
	o.raw = append(o.raw, chunk...)
	if len(o.raw) > 2*historyBytes {
		o.raw = slices.Clone(o.raw[len(o.raw)-historyBytes:])
	}
	for ch := range o.attached {
		select {
		case ch <- chunk:
		default:
			delete(o.attached, ch)
			close(ch)
		}
	}
	lines := o.splitter.Split(chunk, o.now())
	o.keep(lines)
	o.mu.Unlock()

	o.hand(lines)
}

// end gives out the line held with no newline, ends every follower and client
// attached, drops the raw history no client can attach to anymore, closes the
// log, and returns the texts of the last lines a job.crashed carries.
func (o *output) end() []string {
	o.mu.Lock()
	lines := o.splitter.Flush(o.now())
	o.keep(lines)
	o.ended = true
	for f := range o.followers {
		close(f.lines)
	}
	for ch := range o.attached {
		close(ch)
	}
	clear(o.followers)
	clear(o.attached)
	o.raw = nil
	out := o.out
	// The start's client waited for this end: a line noted after it is the job's, not that client's.
	o.out = nil
	last := []string{}
	for _, line := range o.lines[max(0, len(o.lines)-lastLines):] {
		// A crash's last lines are the job's: the run mark is wtm's.
		if line.Text != run.Mark {
			last = append(last, line.Text)
		}
	}
	o.mu.Unlock()

	o.handTo(lines, out)
	if o.log != nil {
		o.log.Close()
	}
	return last
}

// keep adds lines to the last ones and hands each to the followers; the lock
// is held.
func (o *output) keep(lines []run.Line) {
	if len(lines) == 0 {
		return
	}
	for _, line := range lines {
		o.text += len(line.Text)
	}
	o.lines = append(o.lines, lines...)
	if len(o.lines) > 2*run.MaxTail || o.text > 2*tailBytes {
		cut := 0
		for len(o.lines)-cut > run.MaxTail || o.text > tailBytes {
			o.text -= len(o.lines[cut].Text)
			cut++
		}
		o.lines = slices.Clone(o.lines[cut:])
	}
	last := lines[len(lines)-1]
	o.last.Store(&last)

	for f := range o.followers {
		for _, line := range lines {
			select {
			case f.lines <- line:
				continue
			default:
			}
			f.behind.Store(true)
			delete(o.followers, f)
			close(f.lines)
			break
		}
	}
}

// hand writes lines to the log and the start's client while it waits; the
// lock is not held.
func (o *output) hand(lines []run.Line) {
	o.mu.Lock()
	out := o.out
	o.mu.Unlock()
	o.handTo(lines, out)
}

// handTo writes lines to the log and to out (nil for none); the lock is not
// held.
func (o *output) handTo(lines []run.Line, out func(run.Line)) {
	if len(lines) == 0 {
		return
	}
	if o.log != nil {
		o.log.Write(lines)
	}
	if out != nil {
		for _, line := range lines {
			out(line)
		}
	}
}

// note adds a line of wtm's own to the job's output, as if the job printed it.
func (o *output) note(text string) {
	lines := []run.Line{{At: o.now(), Text: text}}
	o.mu.Lock()
	o.keep(lines)
	o.mu.Unlock()
	o.hand(lines)
}

// beginParams are how a run of the job begins: its process spawned, its log opened, and the mark written first; Before are the last lines of the run before, which a run continuing its log (a restart) keeps above the mark for its readers, as its log does.
type beginParams struct {
	Spawn  func() error
	Open   func() Log
	Mark   string
	Before []run.Line
}

// begin spawns the job's process with the lock held, so nothing it prints
// lands before the mark, then opens its log and writes the mark into it and
// the lines its readers follow, never as the job's last line, nor handed to
// the start's client: it is the log's, not the job's. A spawn that fails opens
// no log.
func (o *output) begin(params beginParams) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := params.Spawn(); err != nil {
		return err
	}

	o.log = params.Open()
	mark := []run.Line{{At: o.now(), Text: params.Mark}}
	last := o.last.Load()
	o.keep(params.Before)
	o.keep(mark)
	o.last.Store(last)
	if o.log != nil {
		o.log.Write(mark)
	}
	return nil
}

// kept returns the last lines kept.
func (o *output) kept() []run.Line {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.lines)
}

func (o *output) lastLine() *run.Line {
	return o.last.Load()
}

// follow returns the last n lines and, when live and the job runs, the
// follower each new one arrives to, with the function that lets it go;
// whether the job has ended.
func (o *output) follow(n int, live bool) ([]run.Line, *follower, func(), bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	past := slices.Clone(o.lines[max(0, len(o.lines)-n):])
	if !live || o.ended {
		return past, nil, func() {}, o.ended
	}
	f := &follower{lines: make(chan run.Line, followQueue)}
	o.followers[f] = struct{}{}
	return past, f, func() { o.drop(f) }, false
}

func (o *output) drop(f *follower) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, open := o.followers[f]; open {
		delete(o.followers, f)
		close(f.lines)
	}
}

// attach returns what the job printed last, raw, and the channel what it
// prints next arrives on, with the function that lets it go.
func (o *output) attach() ([]byte, chan []byte, func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	ch := make(chan []byte, followQueue)
	if o.ended {
		close(ch)
	} else {
		o.attached[ch] = struct{}{}
	}
	return slices.Clone(o.raw[max(0, len(o.raw)-historyBytes):]), ch, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if _, open := o.attached[ch]; open {
			delete(o.attached, ch)
			close(ch)
		}
	}
}

// A terminal is a job's terminal as its clients type into it: what they type
// is queued and written by one goroutine, so a job that never reads its input
// holds no client; what overflows the queue is dropped.
type terminal struct {
	keys chan []byte
}

// newTerminal returns the terminal of process, fed until done is closed.
func newTerminal(process Process, done <-chan struct{}) *terminal {
	t := &terminal{keys: make(chan []byte, inputQueue)}
	go func() {
		for {
			select {
			case keys := <-t.keys:
				_, _ = process.Write(keys)
			case <-done:
				return
			}
		}
	}()
	return t
}

// Write queues p, or drops it when the queue is full; it never fails.
func (t *terminal) Write(p []byte) (int, error) {
	select {
	case t.keys <- bytes.Clone(p):
	default:
	}
	return len(p), nil
}
