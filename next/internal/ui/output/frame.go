// Package output is the presentation kit of every command: human frames and blocks, the progress of a run, failures, plans and the JSON envelope.
package output

import (
	"bytes"
	"io"
)

const (
	accentBar      = "┃"
	accentBarWidth = 1
)

func Frame(w io.Writer, body func(io.Writer)) {
	Blank(w)
	body(barred(w))
	Blank(w)
}

func barred(w io.Writer) io.Writer {
	if !IsTerminal(w) {
		return w
	}
	return &barWriter{w: w, atLineStart: true}
}

type barWriter struct {
	w           io.Writer
	atLineStart bool
	afterCR     bool
}

func (b *barWriter) Write(p []byte) (int, error) {
	var out bytes.Buffer
	for _, c := range p {
		// A CRLF cut across two writes is one break: the bar the \r put back must not be drawn twice.
		if c == '\n' && b.afterCR {
			out.WriteByte(c)
			b.afterCR = false
			continue
		}
		b.afterCR = false
		if b.atLineStart {
			out.WriteString(primary.Render(accentBar))
			b.atLineStart = false
		}
		out.WriteByte(c)
		switch c {
		case '\n':
			b.atLineStart = true
		case '\r':
			b.atLineStart, b.afterCR = true, true
		}
	}
	if _, err := b.w.Write(out.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (b *barWriter) Unwrap() io.Writer { return b.w }
