package output

import (
	"os"

	"fmt"
	"io"
	"strings"

	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// Indent is the standard left padding used by all TUI components.
// Use this to align non-TUI output with TUI prompts.
// The canonical definition lives in styles.Indent; this alias keeps callers unchanged.
const Indent = styles.Indent

// Warning prints the attention line: "  ! message". The glyph is a plain rune
// like every other: it used to be a filled chip, which carried its own padding
// and made an attention line two columns wider — and therefore louder — than
// the failure line under it.
func Warning(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Warning.Render(domain.GlyphAttention), msg)
}

// InfoLine prints a styled key-value pair: "  label  value".
func InfoLine(w io.Writer, label string, value string) {
	fmt.Fprintf(w, "%s%s  %s\n", Indent, styles.Muted.Render(label), value)
}

// SectionTitle prints a bold section title with standard indent.
func SectionTitle(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s\n", Indent, styles.Bold.Render(title))
}

// Success prints a styled success line: "  ✓ message".
func Success(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Success.Render(domain.GlyphSuccess), msg)
}

// Update prints a styled update line: "  ↻ message".
// Mirrors Success but signals that an existing artifact was refreshed.
func Update(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Primary.Render(domain.GlyphUpdate), msg)
}

// Unchanged prints the no-op line: "  = message", muted whole. Use it when an
// artifact already matched the desired state and nothing was written — and for
// an inventory that came back empty, which is the same non-event.
func Unchanged(w io.Writer, msg string) {
	fmt.Fprint(w, UnchangedLine(msg))
}

// UnchangedLine is Unchanged for a formatter that returns a body instead of
// writing one. Those formatters used to answer an empty inventory with a bare
// sentence, which is how one state ended up with five renderings across the
// tree.
func UnchangedLine(msg string) string {
	return fmt.Sprintf("%s%s %s\n", Indent, styles.Muted.Render(domain.GlyphUnchanged), styles.Muted.Render(msg))
}

// Error prints the failure line: "  ✗ message". A msg carrying newlines — a
// subprocess's own output — puts its first line next to the cross and indents
// the rest under it. Those lines are not muted: indentation is what makes them
// subordinate, and muting the detail of a failure hides the half a reader came
// for.
func Error(w io.Writer, msg string) {
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.DangerText.Render(domain.GlyphFailure), lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%s  %s\n", Indent, line)
	}
}

// Loading prints a styled loading/status line: "  › message".
func Loading(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Muted.Render(domain.GlyphProgress), styles.Muted.Render(msg))
}

// Message prints a plain indented message.
func Message(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s\n", Indent, msg)
}

// Blank prints an empty line for vertical spacing.
func Blank(w io.Writer) {
	fmt.Fprintln(w)
}

// Callout prints a bordered notice box with a bold title followed by body lines.
// Use it to surface an optional, non-blocking hint above an interactive flow.
// It emits a raw box with no surrounding blank lines; the caller's frame owns
// the outer vertical padding.
func Callout(w io.Writer, title string, lines []string) {
	rows := append([]string{styles.CalloutTitle.Render(title)}, lines...)
	// Bounded to the terminal: a box grows to its longest line, and one line
	// naming eight jobs made a 178-column frame that wrapped into mush on any
	// normal window.
	box := styles.Callout.Width(calloutWidth(w, lines)).Render(strings.Join(rows, "\n"))
	fmt.Fprintf(w, "%s\n", box)
}

// calloutWidth is what a callout's body may fill: the terminal less the box's
// own margin, border and padding. Zero when there is no terminal to measure —
// a pipe or a test — which leaves the box at its content's width, as before.
//
// The cap is on the empty space, never on the content: a short notice is not
// stretched across a very wide window, but a body already wider than the cap
// takes the room it needs rather than being wrapped into mush. A table is the
// case that made the difference — its lines are columns, and a wrapped column
// is not a narrower table but an unreadable one.
func calloutWidth(w io.Writer, lines []string) int {
	cols := TerminalWidthOf(w)
	if cols <= 0 {
		return 0
	}
	return min(cols-domain.CalloutChrome, max(domain.CalloutMaxWidth, widestRow(lines)))
}

// widestRow measures the body, never the title: the title is styled, and the
// escape sequences in it would be counted as room the box does not need.
func widestRow(lines []string) int {
	widest := 0
	for _, line := range lines {
		widest = max(widest, len([]rune(line)))
	}
	return widest
}

// TerminalWidthOf is what a surface has to draw in, zero when the stream is no
// terminal. It measures the stream actually written to: a surface that draws on
// stderr and measures stdout gets 0 the moment stdout is redirected,
// which is the common `wtm create > out.txt`; every line it then draws too wide
// wraps, and a block redrawn in place cannot count the rows it took.
func TerminalWidthOf(w io.Writer) int {
	stream, barred := unwrapStream(w)
	file, ok := stream.(*os.File)
	if !ok {
		return 0
	}
	cols, _, err := term.GetSize(int(file.Fd()))
	if err != nil || cols <= 0 {
		return 0
	}
	if barred {
		cols -= domain.AccentBarWidth
	}
	return cols
}

// unwrapStream reads through the accent bar to the stream underneath, and says
// whether it went through one. A barred writer is the terminal it wraps, less
// the column the bar takes.
func unwrapStream(w io.Writer) (io.Writer, bool) {
	barred := false
	for {
		unwrapper, ok := w.(interface{ Unwrap() io.Writer })
		if !ok {
			return w, barred
		}
		w, barred = unwrapper.Unwrap(), true
	}
}

// IsTerminal reports whether w is a terminal this process may repaint. A pipe, a
// buffer or a file is not: a surface that moves the cursor there writes escape
// sequences into someone's log.
func IsTerminal(w io.Writer) bool {
	stream, _ := unwrapStream(w)
	file, ok := stream.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// TallyPart is one count of a result summary. A zero count is dropped: a
// conclusion counts what happened, never what did not.
type TallyPart struct {
	Count int
	Label string
}

// Tally renders the counted half of a multi-item conclusion — "3 applied ·
// 1 skipped". It is what replaces one line per success: the reader checks the
// count, and only the exceptions are worth a line of their own.
func Tally(parts ...TallyPart) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Count == 0 {
			continue
		}
		kept = append(kept, fmt.Sprintf(domain.TallyPartFmt, part.Count, part.Label))
	}
	return strings.Join(kept, domain.TallySeparator)
}

type NextStepParams struct {
	// Command is ready to run as printed; Note says what it does, when the
	// command alone does not.
	Command string
	Note    string
}

// NextStep prints the one forward-pointing line of a conclusion: "→ wtm go x".
// It is the only shape a hint takes anywhere in the CLI — a command in bold
// after an arrow — so a reader learns once where to look for what to do next.
func NextStep(w io.Writer, params NextStepParams) {
	fmt.Fprintln(w, NextStepLine(params))
}

// NextStepLine is NextStep for a caller composing a body rather than writing
// one — a recap built as a single string, printed once the surface it belongs
// to has given the terminal back. Those used to hand-roll their own hint, which
// is how "what to do next" ended up with five renderings.
func NextStepLine(params NextStepParams) string {
	return styles.NextStepLine(styles.NextStepParams{Command: params.Command, Note: params.Note})
}

// Section prints a bold title above indented free lines, with no frame — a
// script, a file's contents, a listing. Rows of `label  value` belong in
// Announce instead, and Callout's bordered frame is reserved for what the reader
// still has to act on. Mixing the two made every outcome look equally urgent,
// which is the same as flagging none of them.
func Section(w io.Writer, title string, lines []string) {
	SectionTitle(w, title)
	for _, line := range lines {
		fmt.Fprintf(w, "%s%s%s\n", Indent, Indent, line)
	}
}

// AnnounceItem is a label-value pair displayed inside an Announce block.
type AnnounceItem struct {
	Label string
	Value string
}

// Announce prints a titled block of `label  value` rows, its labels aligned to
// a common column. It is the shape of anything a reader looks *up* — a plan
// before a picker, a state readout — as against Section, which is a titled
// block of free lines. Blocks that hand-aligned their labels with spaces inside
// a format string belong here: the alignment is the block's business, not the
// wording's.
//
// It emits no surrounding blank lines; the caller's frame owns the outer
// vertical padding.
func Announce(w io.Writer, title string, items []AnnounceItem) {
	SectionTitle(w, title)
	width := 0
	for _, item := range items {
		width = max(width, len(item.Label))
	}
	for _, item := range items {
		// Labels are plain ASCII, so byte length is printable width; the padding is
		// computed before rendering because the style adds bytes that take no room.
		pad := strings.Repeat(" ", width-len(item.Label))
		fmt.Fprintf(w, "%s%s%s  %s\n", Indent, styles.Muted.Render(item.Label), pad, item.Value)
	}
}
