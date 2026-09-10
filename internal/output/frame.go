package output

import (
	"bytes"
	"io"
	"reflect"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// Frame renders a command's human output as one block: the accent bar that marks
// it as wtm's own, with exactly one blank line before and one after.
//
// The body writes to the writer it is handed, never to the one Frame was given —
// that is what puts the bar on every line, in one place, the way the padding is
// applied in one place.
//
// JSON and machine-readable (shell-eval) paths must NOT be framed — they emit
// flush output and never call Frame/FrameStart/FrameEnd.
func Frame(w io.Writer, render func(io.Writer)) {
	FrameStart(w)
	render(Barred(w))
	FrameEnd(w)
}

// A surface remembers where its last block left the cursor, because the blank
// line closing one block and the blank opening the next are the same line on
// screen. A caller that has to decide whether to open one — a run reporting
// while it is still running — cannot answer from what it did itself: the block
// beside it is opened by code that never sees it, `run up`'s own frame around a
// job's output being the case that made this necessary.
//
// stdout and stderr are one surface when both are the same terminal: a reader
// looking at it sees one column of blocks, whichever stream wrote them.
type surfaceState int

const (
	surfaceFresh surfaceState = iota
	// surfaceBoundary: a blank line is the last thing written, so the next block
	// opens on it rather than under a second one.
	surfaceBoundary
	surfaceOpen
)

var (
	surfacesMu sync.Mutex
	surfaces   = map[any]surfaceState{}
	// theTerminal keys every stream that reaches the terminal, since they all
	// draw on the one the reader is watching.
	theTerminal = new(int)
)

// surfaceOf keys w by what a reader actually sees. An untracked writer — one
// that cannot be a map key — reads fresh and records nothing, which is the
// behaviour every caller had before a surface remembered anything.
func surfaceOf(w io.Writer) (any, bool) {
	stream, _ := unwrapStream(w)
	if IsTerminal(stream) {
		return theTerminal, true
	}
	if !reflect.TypeOf(stream).Comparable() {
		return nil, false
	}
	return stream, true
}

func surfaceAt(w io.Writer) surfaceState {
	key, ok := surfaceOf(w)
	if !ok {
		return surfaceFresh
	}
	surfacesMu.Lock()
	defer surfacesMu.Unlock()
	return surfaces[key]
}

func markSurface(w io.Writer, state surfaceState) {
	key, ok := surfaceOf(w)
	if !ok {
		return
	}
	surfacesMu.Lock()
	defer surfacesMu.Unlock()
	surfaces[key] = state
}

// BlockOpen reports whether w draws on a surface whose block is still open, so
// a caller adds to it instead of opening a second one beside it.
func BlockOpen(w io.Writer) bool {
	return surfaceAt(w) == surfaceOpen
}

// FrameStart writes the single leading blank line of a command frame. Pair it
// with exactly one FrameEnd, and wrap the body's writer in Barred yourself.
// Prefer Frame; use the explicit pair only for streaming, spinner-driven, or
// split-stream commands whose body cannot be a single closure — content that
// starts on stderr (a plan or spinner) and finishes on stdout (the result), or
// output interleaved with live spinners.
func FrameStart(w io.Writer) {
	// A surface already at a boundary carries the blank this frame would write:
	// the block that closed there and this one are separated by one line, not
	// two. An open block gets that one line as its close and this one's open at
	// once, which is the same rule read from the other side.
	if surfaceAt(w) != surfaceBoundary {
		Blank(w)
	}
	markSurface(w, surfaceOpen)
}

// FrameEnd writes the single trailing blank line of a command frame. For
// split-stream commands it may be called on a different writer than FrameStart
// (e.g. FrameStart(stderr) … FrameEnd(stdout)).
func FrameEnd(w io.Writer) {
	Blank(w)
	markSurface(w, surfaceBoundary)
}

// Barred wraps w so every line of a block carries the accent bar. On anything
// but a terminal it hands w back unchanged: a pipe, a CI log or a redirection
// gets the bare text, so a grep over the log never has to know about the bar.
func Barred(w io.Writer) io.Writer {
	if !IsTerminal(w) {
		return w
	}
	return &barWriter{w: w, atLineStart: true}
}

// barWriter prefixes each line as it goes rather than buffering the block: a
// phase that streams — a spinner, a hook, a job's output — has to be barred
// while it is still being written.
type barWriter struct {
	w           io.Writer
	atLineStart bool
	// afterCR says the last byte written was a carriage return, so a newline
	// arriving next closes the same break rather than opening a second one. It is
	// state and not a lookahead because a streamed body is cut wherever the
	// producer flushed: a CRLF straddling two writes would otherwise put a bar
	// between the two halves, and the CR would paint it over the line it just
	// finished.
	afterCR bool
}

func (b *barWriter) Write(p []byte) (int, error) {
	var out bytes.Buffer
	for _, c := range p {
		// A CRLF is one break, not two: the row was already marked by the \r.
		if c == '\n' && b.afterCR {
			out.WriteByte(c)
			b.afterCR = false
			continue
		}
		b.afterCR = false
		if b.atLineStart {
			out.WriteString(styles.Primary.Render(domain.AccentBarGlyph))
			b.atLineStart = false
		}
		out.WriteByte(c)
		// A carriage return puts the cursor back in column zero, over the bar this
		// line already carries, so the row has to be marked again.
		if c == '\n' {
			b.atLineStart = true
		}
		if c == '\r' {
			b.atLineStart, b.afterCR = true, true
		}
	}
	if _, err := b.w.Write(out.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Unwrap gives back the writer underneath the bar, so a caller that has to know
// what it is really writing to — is it a terminal, how wide is it — reads the
// file rather than the wrapper. Without it a barred hook phase would decide it
// was writing to a pipe and stream its whole output instead of a tail.
func (b *barWriter) Unwrap() io.Writer { return b.w }
