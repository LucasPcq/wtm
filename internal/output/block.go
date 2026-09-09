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

// Warning prints a styled warning line: "  ! message".
func Warning(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.BadgeWarning.Render("!"), styles.Warning.Render(msg))
}

// Danger prints a styled failure line in the danger theme: "  ! message"
// (red badge + red text) — the red counterpart of Warning, for failures that
// should read like a skip rather than a hard crash.
func Danger(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.BadgeDanger.Render("!"), styles.DangerText.Render(msg))
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
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Success.Render("✓"), msg)
}

// Update prints a styled update line: "  ↻ message".
// Mirrors Success but signals that an existing artifact was refreshed.
func Update(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Primary.Render("↻"), msg)
}

// Unchanged prints a muted no-op line: "  = message".
// Use it when an artifact already matched the desired state and nothing was written.
func Unchanged(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Muted.Render("="), styles.Muted.Render(msg))
}

// Error prints a styled error line: "  ✗ message".
// If msg contains newlines (e.g. captured subprocess output), only the first
// line rides next to the cross; the rest is indented underneath so terminal
// rendering stays readable.
func Error(w io.Writer, msg string) {
	cross := styles.DangerText.Render("✗")
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	fmt.Fprintf(w, "%s%s %s\n", Indent, cross, lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(w, "%s  %s\n", Indent, styles.Muted.Render(line))
	}
}

// Loading prints a styled loading/status line: "  › message".
func Loading(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s %s\n", Indent, styles.Muted.Render("›"), styles.Muted.Render(msg))
}

// Message prints a plain indented message.
func Message(w io.Writer, msg string) {
	fmt.Fprintf(w, "%s%s\n", Indent, msg)
}

// Blank prints an empty line for vertical spacing.
func Blank(w io.Writer) {
	fmt.Fprintln(w)
}

// HooksSection prints a leading blank and a bold title above a phase of streamed
// hook output (e.g. "Running on_create hooks"), so lifecycle hooks read as a
// distinct, labelled phase instead of loose lines in the middle of the command.
func HooksSection(w io.Writer, title string) {
	Blank(w)
	SectionTitle(w, title)
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
	box := styles.Callout.Width(calloutWidth(lines)).Render(strings.Join(rows, "\n"))
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
func calloutWidth(lines []string) int {
	cols, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 {
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
	file, ok := w.(*os.File)
	if !ok {
		return 0
	}
	cols, _, err := term.GetSize(int(file.Fd()))
	if err != nil || cols <= 0 {
		return 0
	}
	return cols
}

// IsTerminal reports whether w is a terminal this process may repaint. A pipe, a
// buffer or a file is not: a surface that moves the cursor there writes escape
// sequences into someone's log.
func IsTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// Section prints a bold title above indented lines, with no frame. It is what a
// command reports having done; Callout's bordered frame is reserved for what the
// reader still has to act on. Mixing the two made every outcome look equally
// urgent, which is the same as flagging none of them.
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

// Announce prints a raw block with a bold section title followed by indented
// key-value rows. Use it before an interactive picker to describe what is about
// to happen. It emits no surrounding blank lines; the caller's frame owns the
// outer vertical padding.
func Announce(w io.Writer, title string, items []AnnounceItem) {
	SectionTitle(w, title)
	for _, item := range items {
		InfoLine(w, item.Label, item.Value)
	}
}
