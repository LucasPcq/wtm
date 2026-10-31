package run

import (
	"regexp"
	"strings"
	"time"
)

// MaxTail is the most lines a client may ask of a job's past: the daemon keeps
// that many of each job it runs.
const MaxTail = 1000

// maxPending is how much of a line with no newline yet a Splitter holds before
// it gives it out as a line of its own: a job redrawing one line for ever
// (a progress bar) must not grow it without end.
const maxPending = 64 << 10

// A Line is one line a job printed, as the daemon read it: when, and its text,
// its terminal escapes, carriage-return redraws and control characters gone.
// A job's stdout and stderr are one stream, a terminal's.
type Line struct {
	At   time.Time
	Text string
}

// A Splitter cuts what a job prints into Lines, holding the end of a line
// until its newline arrives. The zero Splitter is ready to use; it is not safe
// for concurrent use.
type Splitter struct {
	pending string
}

// Split returns the lines chunk completes, each stamped at, a blank one left
// out.
func (s *Splitter) Split(chunk []byte, at time.Time) []Line {
	segments := strings.Split(s.pending+string(chunk), "\n")
	var lines []Line
	for _, segment := range segments[:len(segments)-1] {
		lines = appendLine(lines, Line{At: at, Text: clean(segment)})
	}
	s.pending = lastRedraw(segments[len(segments)-1])
	if len(s.pending) > maxPending {
		lines = appendLine(lines, Line{At: at, Text: clean(s.pending)})
		s.pending = ""
	}

	return lines
}

// Flush returns the line held with no newline, stamped at, if any: what a job
// printed last before it ended.
func (s *Splitter) Flush(at time.Time) []Line {
	pending := s.pending
	s.pending = ""

	return appendLine(nil, Line{At: at, Text: clean(pending)})
}

func appendLine(lines []Line, line Line) []Line {
	if line.Text == "" {
		return lines
	}

	return append(lines, line)
}

var terminalEscape = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?|\x1b\[[0-9;?:<=>]*[ -/]*[@-~]|\x1b[()*+#][0-9A-Za-z]|\x1b[@-Z\\-_]`)

// clean returns raw as a terminal shows it once drawn: its last redraw, no
// escape sequence, no control character but tabs, no trailing blank.
func clean(raw string) string {
	text := terminalEscape.ReplaceAllString(lastRedraw(raw), "")
	text = strings.Map(func(r rune) rune {
		if r != '\t' && (r < 0x20 || r == 0x7f) {
			return -1
		}
		return r
	}, text)

	return strings.TrimRight(text, " \t")
}

// lastRedraw returns what follows raw's last carriage return, a trailing one
// aside (a CRLF ending): a line a job redrew is the redraw.
func lastRedraw(raw string) string {
	end := len(raw)
	if strings.HasSuffix(raw, "\r") {
		end--
	}
	if last := strings.LastIndex(raw[:end], "\r"); last >= 0 {
		return raw[last+1:]
	}

	return raw
}
