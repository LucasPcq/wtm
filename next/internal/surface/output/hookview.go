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
)

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
	view := &hookView{w: params.W, logPath: params.LogPath, header: muted.Render(glyphNext + " " + params.Label)}
	view.repaint()
	return view
}

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

type hookEnd struct {
	Label    string
	Duration time.Duration
	Failed   bool
}

func (v *hookView) finish(end hookEnd) {
	v.flush()
	failed := v.tail
	v.clear()
	result := fmt.Sprintf("%s (%s)", end.Label, duration(end.Duration))
	if !end.Failed {
		successLine(v.w, result)
		return
	}
	errorLine(v.w, result)
	for _, line := range failed {
		message(v.w, indent+truncate(expandTabs(line), max(v.width()-len(indent), hookMinWidth)))
	}
	if v.logPath != "" {
		infoLine(v.w, hookLogLabel, v.logPath)
	}
}

func (v *hookView) notify(notice string) {
	v.notice = notice
	v.repaint()
}

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

func (v *hookView) flush() {
	if v.buf.Len() == 0 {
		return
	}
	rest := v.buf.String()
	v.buf.Reset()
	v.push(rest)
}

func (v *hookView) repaint() {
	v.clear()
	width := v.width()
	fmt.Fprintf(v.w, "%s%s\n", indent, truncate(v.header, width))
	v.painted++
	if v.notice != "" {
		message(v.w, truncate(v.notice, width))
		v.painted++
	}
	for _, line := range v.tail {
		body := truncate(expandTabs(line), max(width-len(indent), hookMinWidth))
		fmt.Fprintf(v.w, "%s%s%s\n", indent, indent, muted.Render(body))
		v.painted++
	}
}

func (v *hookView) clear() {
	if v.painted == 0 {
		return
	}
	fmt.Fprintf(v.w, ansiPrevLine+ansiClearBelow, v.painted)
	v.painted = 0
}

func (v *hookView) width() int {
	cols := terminalWidth(v.w)
	if cols <= 0 {
		return hookFallbackWidth
	}
	return cols - 2*len(indent)
}

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

func duration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
