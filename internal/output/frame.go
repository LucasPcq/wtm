package output

import (
	"bytes"
	"io"

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

// FrameStart writes the single leading blank line of a command frame. Pair it
// with exactly one FrameEnd, and wrap the body's writer in Barred yourself.
// Prefer Frame; use the explicit pair only for streaming, spinner-driven, or
// split-stream commands whose body cannot be a single closure — content that
// starts on stderr (a plan or spinner) and finishes on stdout (the result), or
// output interleaved with live spinners.
func FrameStart(w io.Writer) {
	Blank(w)
}

// FrameEnd writes the single trailing blank line of a command frame. For
// split-stream commands it may be called on a different writer than FrameStart
// (e.g. FrameStart(stderr) … FrameEnd(stdout)).
func FrameEnd(w io.Writer) {
	Blank(w)
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
}

func (b *barWriter) Write(p []byte) (int, error) {
	var out bytes.Buffer
	for i, c := range p {
		if b.atLineStart {
			out.WriteString(styles.Primary.Render(domain.AccentBarGlyph))
			b.atLineStart = false
		}
		out.WriteByte(c)
		// A carriage return puts the cursor back in column zero, over the bar this
		// line already carries, so the row has to be marked again. A CRLF is one
		// break, not two: marking between the two would leave a bar on its own.
		if c == '\n' || (c == '\r' && !(i+1 < len(p) && p[i+1] == '\n')) {
			b.atLineStart = true
		}
	}
	if _, err := b.w.Write(out.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}
