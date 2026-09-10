package output

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// HookView shows a hook phase without keeping it. The running hook draws a
// bounded tail of its own output, redrawn in place; when it finishes the tail is
// erased and one result line takes its place.
//
// It is the answer to the two things a streamed hook has to do at once. Silence
// for forty seconds reads as a hang, so the stream has to be visible while it
// runs — but a scrollback holding a whole install is a screen nobody rereads, so
// it must not survive. What does survive is the log file: a hook that failed is
// exactly when the output matters, and it is there whether or not the tail was
// long enough to hold it.
//
// It is for a terminal it may repaint. A pipe, a CI log or a JSON run gets the
// raw stream instead — see CLIPresenter.HookPhase.
//
// The cost, taken knowingly: the hook writes into a pipe rather than onto the
// terminal, so it sees no tty. Tools that colour or animate only for one fall
// back to their plain output, and those on C stdio buffer by block instead of by
// line — a tail that fills in bursts. Keeping the tty would mean allocating a
// pty for every hook, which is a great deal of machinery for output this
// deliberately throws away.
type HookView struct {
	w io.Writer
	// line is where every composed line goes, barred when the phase draws inside
	// a block. The escapes of clear() go to w instead: they move the cursor
	// rather than open a row, and a bar drawn before one lands on the line the
	// cursor is about to leave — then survives the erase, one column off.
	line    io.Writer
	logPath string
	log     io.Writer
	buf     bytes.Buffer
	// rewrite says the last segment ended on a carriage return, so the next one
	// overwrites it rather than following it — that is what a progress bar means.
	rewrite bool
	tail    []string
	header  string
	painted int
}

type HookViewParams struct {
	W io.Writer
	// Log is the open log, or nil. The view neither opens nor closes it: the log
	// is kept whatever the surface draws, so it outlives any one view — see
	// HookLog.
	Log io.Writer
	// LogPath is what the failure line points the reader at. Empty prints no such
	// line.
	LogPath string
	// Bar draws the phase inside the accent bar. What a phase leaves behind — a
	// result line per hook, and the record of a failed one — is kept, so it
	// belongs in the block; only the live tail is thrown away.
	Bar bool
}

func NewHookView(params HookViewParams) *HookView {
	view := &HookView{w: params.W, line: params.W, logPath: params.LogPath}
	if params.Bar {
		view.line = Barred(params.W)
	}
	if params.Log != nil {
		view.log = params.Log
	}
	return view
}

// HookLog opens the file a hook phase's whole output is kept in. Every surface
// opens it, not only one that collapses the stream: a run whose output the
// reader could not watch — a pipe, a CI log, a JSON run, a quiet one — is
// exactly the one whose record has to survive. Nil when there is no path or it
// cannot be opened: the log is what makes the collapse safe, never what makes
// the hook run.
func HookLog(path string) *os.File {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil
	}
	return file
}

// Write splits on carriage returns as well as newlines. A hook that only ever
// rewrites one row — a download, a spinner, a bundler — would otherwise never
// complete a line, leaving the tail empty for its whole run while the buffer
// grew to hold every frame it drew.
func (v *HookView) Write(p []byte) (int, error) {
	if v.log != nil {
		_, _ = v.log.Write(p)
	}
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

// OnHook opens the tail on the hook that is starting, and closes it on the one
// that finished — the erase and the result line are one repaint, so the phase
// never flickers between them.
func (v *HookView) OnHook(beat domain.HookBeat) {
	if beat.Started {
		v.header = styles.Muted.Render(domain.HookGlyphStart + " " + rules.HookLabel(beat))
		v.tail, v.rewrite = nil, false
		v.repaint()
		return
	}

	v.flush()
	failed := v.tail
	v.clear()
	v.header, v.tail, v.rewrite = "", nil, false

	if beat.Err == "" {
		Success(v.line, rules.HookResultLabel(beat))
		return
	}

	Error(v.line, rules.HookResultLabel(beat))
	for _, line := range failed {
		v.tailLine(line)
	}
	// stderr reached the tail as it was produced, so only what the bounded tail
	// dropped is printed again — and it is the line the reader came for.
	for _, line := range rules.HookStderrBeyondTail(rules.HookStderrBeyondTailParams{Stderr: beat.Stderr, Tail: failed}) {
		v.tailLine(line)
	}
	if v.logPath != "" {
		InfoLine(v.line, domain.HookLogTailLabel, v.logPath)
	}
}

// Close erases whatever tail is still on screen. A phase that ended on an error
// before its last hook reported has one, and leaving it there would put a
// half-drawn viewport above the error message.
func (v *HookView) Close() {
	v.flush()
	v.clear()
}

func (v *HookView) push(line string) {
	if v.rewrite && len(v.tail) > 0 {
		v.tail[len(v.tail)-1] = line
	} else {
		v.tail = append(v.tail, line)
	}
	if len(v.tail) > domain.HookViewTailLines {
		v.tail = v.tail[len(v.tail)-domain.HookViewTailLines:]
	}
	v.repaint()
}

func (v *HookView) flush() {
	if v.buf.Len() == 0 {
		return
	}
	rest := v.buf.String()
	v.buf.Reset()
	v.push(rest)
}

// repaint rewrites the whole block in place. Every line is cut to the width of
// the stream being drawn on: the cursor is moved back by as many rows as were
// printed, and a line that wrapped would have taken two of them.
func (v *HookView) repaint() {
	v.clear()
	width := v.width()
	if v.header != "" {
		fmt.Fprintf(v.line, "%s%s\n", Indent, styles.Truncate(styles.TruncateParams{Value: v.header, Width: width}))
		v.painted++
	}
	for _, line := range v.tail {
		v.paintTail(line, width-len([]rune(Indent)))
	}
}

func (v *HookView) paintTail(line string, width int) {
	body := styles.Truncate(styles.TruncateParams{Value: styles.ExpandTabs(line), Width: max(width, domain.HookViewMinWidth)})
	fmt.Fprintf(v.line, "%s%s%s\n", Indent, Indent, styles.Muted.Render(body))
	v.painted++
}

// tailLine prints one line of the record a failed hook leaves behind. It goes
// through the same cut as the live tail: this is the only path whose output
// survives, so it is the one where a wrapped or self-erasing line shows.
func (v *HookView) tailLine(line string) {
	body := styles.Truncate(styles.TruncateParams{Value: styles.ExpandTabs(line), Width: max(v.width()-len([]rune(Indent)), domain.HookViewMinWidth)})
	// Not muted: this is the record a failed hook leaves, and it is the half the
	// reader came for. The indent is what makes it subordinate.
	Message(v.line, Indent+body)
}

func (v *HookView) clear() {
	if v.painted == 0 {
		return
	}
	fmt.Fprintf(v.w, domain.AnsiCursorUpFmt+domain.AnsiClearBelow, v.painted)
	v.painted = 0
}

func (v *HookView) width() int {
	cols := TerminalWidthOf(v.line)
	if cols <= 0 {
		return domain.HookViewFallbackWidth
	}
	return cols - 2*len([]rune(Indent))
}

// cutSegment takes the next display line off the buffer, whichever of the two
// terminators ends it, and reports which one did.
func cutSegment(buf *bytes.Buffer) (segment string, terminator byte, ok bool) {
	body := buf.Bytes()
	at := bytes.IndexAny(body, "\n\r")
	if at < 0 {
		return "", 0, false
	}
	segment = string(body[:at])
	terminator = body[at]
	buf.Next(at + 1)
	// A CRLF is one terminator, not two: the empty segment its \n would produce
	// would blank the row the \r just meant to rewrite.
	if terminator == '\r' && buf.Len() > 0 && buf.Bytes()[0] == '\n' {
		terminator = '\n'
		buf.Next(1)
	}
	return segment, terminator, true
}
