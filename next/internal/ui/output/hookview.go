package output

import (
	"bytes"
	"fmt"
	"io"
	"time"
)

const (
	hookTailLines     = 8
	hookFallbackWidth = 100
	hookMinWidth      = 8
	hookLogLabel      = "full output:"
	ansiPrevLine      = "\x1b[%dF"
	ansiClearBelow    = "\x1b[J"
	ansiClearLine     = "\x1b[2K"
)

// A hookView draws a running hook live: a header, a notice when the operation is cancelling, and the last lines the hook printed, all redrawn in place as lines arrive.
// It is an [io.Writer] the hook's output is written to.
type hookView struct {
	w       io.Writer
	logPath string
	buf     bytes.Buffer
	rewrite bool
	tail    []string
	header  string
	notice  string
	painted int
}

type hookViewParams struct {
	W       io.Writer
	Label   string
	LogPath string
}

func newHookView(params hookViewParams) *hookView {
	view := &hookView{w: params.W, logPath: params.LogPath, header: StylesOf(params.W).Muted.Render(glyphNext + " " + params.Label)}
	view.repaint()
	return view
}

// Write adds the lines p completes to the view, a line ended by a carriage return rewritten by the next, and keeps an unfinished line until it ends.
func (v *hookView) Write(p []byte) (int, error) {
	v.buf.Write(p)

	for {
		segment, terminator, ok := cutSegment(&v.buf)
		if !ok {
			return len(p), nil
		}
		v.push(segment)
		v.rewrite = terminator == '\r'
	}
}

// finish replaces the view with one line of its result: done with its duration, or failed with the last lines the hook printed and the path of its full log.
func (v *hookView) finish(label string, took time.Duration, failed bool) {
	v.flush()
	tail := v.tail
	v.clear()

	result := fmt.Sprintf("%s (%s)", label, Duration(took))
	if !failed {
		SuccessLine(v.w, result)
		return
	}

	ErrorLine(v.w, result)
	for _, line := range tail {
		Message(v.w, Indent+Truncate(expandTabs(line), max(BlockWidth(v.w)-len(Indent), hookMinWidth)))
	}
	if v.logPath != "" {
		InfoLine(v.w, Info{Label: hookLogLabel, Value: v.logPath})
	}
}

func (v *hookView) notify(notice string) {
	v.notice = notice
	v.repaint()
}

// close erases the view, leaving nothing of it on the screen.
func (v *hookView) close() {
	v.flush()
	v.clear()
}

func (v *hookView) push(line string) {
	if v.rewrite && len(v.tail) > 0 {
		v.tail[len(v.tail)-1] = line
	} else {
		v.tail = append(v.tail, line)
	}

	if len(v.tail) > hookTailLines {
		v.tail = v.tail[len(v.tail)-hookTailLines:]
	}

	v.repaint()
}

// flush adds the unfinished line the view still keeps, if any.
func (v *hookView) flush() {
	if v.buf.Len() == 0 {
		return
	}
	rest := v.buf.String()
	v.buf.Reset()
	v.push(rest)
}

// repaint draws the view again where it stood, counting the rows it draws so that the next clear erases them all.
func (v *hookView) repaint() {
	v.clear()
	width := BlockWidth(v.w)

	fmt.Fprintf(v.w, "%s%s\n", Indent, Truncate(v.header, width))
	v.painted++
	if v.notice != "" {
		Message(v.w, Truncate(v.notice, width))
		v.painted++
	}

	for _, line := range v.tail {
		body := Truncate(expandTabs(line), max(width-len(Indent), hookMinWidth))
		fmt.Fprintf(v.w, "%s%s%s\n", Indent, Indent, StylesOf(v.w).Muted.Render(body))
		v.painted++
	}
}

func (v *hookView) clear() {
	EraseLines(v.w, v.painted)
	v.painted = 0
}

// EraseLines erases the last n lines drawn on w, the cursor left where the first of them began; none erases nothing.
func EraseLines(w io.Writer, n int) {
	if n > 0 {
		fmt.Fprintf(w, ansiPrevLine+ansiClearBelow, n)
	}
}

// cutSegment removes from buf its first line and returns it with the byte that ended it, '\n' or '\r'; it reports false, removing nothing, when buf holds no complete line.
func cutSegment(buf *bytes.Buffer) (string, byte, bool) {
	body := buf.Bytes()
	at := bytes.IndexAny(body, "\n\r")
	if at < 0 {
		return "", 0, false
	}

	segment, terminator := string(body[:at]), body[at]
	buf.Next(at + 1)

	// A CRLF is one terminator: the empty segment its \n would cut would blank the row the \r meant to rewrite.
	if terminator == '\r' && buf.Len() > 0 && buf.Bytes()[0] == '\n' {
		terminator = '\n'
		buf.Next(1)
	}
	return segment, terminator, true
}

// Duration returns d in milliseconds under a second, else in seconds to a tenth.
func Duration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
