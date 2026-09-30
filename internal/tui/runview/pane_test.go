package runview

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

func newTestPane(t *testing.T) *Pane {
	t.Helper()
	return NewPane(PaneParams{Size: PaneSize{Cols: 20, Rows: 3}, ScrollbackSize: 10})
}

func write(t *testing.T, p *Pane, chunks ...string) {
	t.Helper()
	for _, chunk := range chunks {
		if _, err := p.Write([]byte(chunk)); err != nil {
			t.Fatalf("write %q: %v", chunk, err)
		}
	}
}

func plainLines(t *testing.T, p *Pane) []string {
	t.Helper()
	lines := strings.Split(ansi.Strip(p.Render()), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

func TestPaneRenderKeepsTruecolor(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "\x1b[38;2;255;0;0mred\x1b[m")

	out := p.Render()
	if !strings.Contains(out, "38;2;255;0;0") {
		t.Fatalf("truecolor sequence dropped from %q", out)
	}
}

func TestPaneRenderKeepsIndexedColor(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "\x1b[31mred\x1b[0m plain")

	out := p.Render()
	if !strings.Contains(out, "\x1b[31m") {
		t.Fatalf("indexed color dropped from %q", out)
	}
	if got := plainLines(t, p)[0]; got != "red plain" {
		t.Fatalf("line 0 = %q, want %q", got, "red plain")
	}
}

func TestPaneRenderGivesWideRunesTwoCells(t *testing.T) {
	cases := map[string]struct {
		input string
		width int
	}{
		"cjk":   {input: "漢a", width: 3},
		"emoji": {input: "🚀ok", width: 4},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := newTestPane(t)
			write(t, p, tc.input)

			line := plainLines(t, p)[0]
			if got := ansi.StringWidth(line); got != tc.width {
				t.Fatalf("width of %q = %d, want %d", line, got, tc.width)
			}
		})
	}
}

func TestPaneRedrawnProgressBarStaysOneLine(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "build 10%\r", "build 55%\x1b[K\r", "build 100%\x1b[K")

	lines := plainLines(t, p)
	if lines[0] != "build 100%" {
		t.Fatalf("line 0 = %q, want %q", lines[0], "build 100%")
	}
	if lines[1] != "" || lines[2] != "" {
		t.Fatalf("carriage returns leaked extra lines: %q", lines)
	}
}

func TestPaneEraseScreenClearsGridAndKeepsHistory(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "before\r\n", "\x1b[2J\x1b[H", "after")

	lines := plainLines(t, p)
	if lines[0] != "after" {
		t.Fatalf("line 0 = %q, want %q", lines[0], "after")
	}
	if strings.Contains(strings.Join(lines, "\n"), "before") {
		t.Fatalf("erased content still on the grid: %q", lines)
	}

	p.ScrollUp(p.term.ScrollbackLen())
	if !strings.Contains(ansi.Strip(p.Render()), "before") {
		t.Fatalf("erased content lost instead of pushed to scrollback: %q", p.Render())
	}
}

func TestPaneAbsoluteCursorAddressing(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "one\r\ntwo\r\nthree", "\x1b[1;1Htop", "\x1b[3;1Hbottom")

	want := []string{"top", "two", "bottom"}
	if got := plainLines(t, p); !equalLines(got, want) {
		t.Fatalf("grid = %q, want %q", got, want)
	}
}

func TestPaneWriteSplitMidEscapeSequence(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "\x1b[38;2;0;25", "5;0mgreen\x1b[m")

	out := p.Render()
	if !strings.Contains(out, "38;2;0;255;0") {
		t.Fatalf("split truecolor sequence lost from %q", out)
	}
	if got := plainLines(t, p)[0]; got != "green" {
		t.Fatalf("line 0 = %q, want %q — escape bytes leaked as text", got, "green")
	}
}

func TestPaneResizeKeepsContentOnce(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "hello\r\n")

	if changed := p.Resize(PaneSize{Cols: 40, Rows: 5}); !changed {
		t.Fatal("Resize reported no change on a new size")
	}
	if got := p.Size(); got != (PaneSize{Cols: 40, Rows: 5}) {
		t.Fatalf("Size() = %+v, want 40x5", got)
	}

	lines := plainLines(t, p)
	if len(lines) != 5 {
		t.Fatalf("rendered %d lines, want 5", len(lines))
	}
	if count := strings.Count(strings.Join(lines, "\n"), "hello"); count != 1 {
		t.Fatalf("%q holds %d copies of hello, want 1", lines, count)
	}
}

// x/vt drops the bottom rows on a shrink and never gives them back, so the
// pane pushes what would go into its scrollback first: the newest output stays
// on screen and the oldest is still one page-up away.
func TestPaneShrinkKeepsTheNewestRowsAndBanksTheOldest(t *testing.T) {
	p := NewPane(PaneParams{Size: PaneSize{Cols: 20, Rows: 6}, ScrollbackSize: 100})
	for i := range 20 {
		write(t, p, fmt.Sprintf("line %02d\r\n", i))
	}

	p.Resize(PaneSize{Cols: 20, Rows: 3})

	shown := strings.Join(plainLines(t, p), "\n")
	if !strings.Contains(shown, "line 19") {
		t.Fatalf("pane = %q, want the newest line the job printed", shown)
	}
	if strings.Contains(shown, "line 15") {
		t.Fatalf("pane = %q, want the shorter screen to hold the tail only", shown)
	}

	p.ScrollUp(4)
	if banked := strings.Join(plainLines(t, p), "\n"); !strings.Contains(banked, "line 15") {
		t.Fatalf("scrollback = %q, want the rows the shrink took", banked)
	}
}

func TestPaneResizeReportsNoChangeOnSameSize(t *testing.T) {
	p := newTestPane(t)
	if changed := p.Resize(PaneSize{Cols: 20, Rows: 3}); changed {
		t.Fatal("Resize reported a change on the current size")
	}
}

func TestPaneDefaultsItsSize(t *testing.T) {
	p := NewPane(PaneParams{})
	if got := p.Size(); got != (PaneSize{Cols: 80, Rows: 24}) {
		t.Fatalf("Size() = %+v, want the 80x24 default", got)
	}
}

// The scrollback budget is what a pane costs in memory: left to itself the
// emulator keeps ten times as many lines.
func TestPaneRetainsNoMoreThanItsScrollbackBudget(t *testing.T) {
	p := NewPane(PaneParams{})

	write(t, p, strings.Repeat("line\r\n", domain.JobPaneScrollbackLines*2))

	if got := p.term.ScrollbackLen(); got != domain.JobPaneScrollbackLines {
		t.Fatalf("scrollback holds %d lines, want the %d it budgets for", got, domain.JobPaneScrollbackLines)
	}
}

func TestPaneScrollUpThenDownReturnsToLive(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "\x1b[38;2;10;20;30mtagged\x1b[m\r\n")
	for i := range 8 {
		write(t, p, fmt.Sprintf("line%d\r\n", i))
	}
	live := p.Render()

	p.ScrollUp(4)
	if p.ScrollOffset() != 4 {
		t.Fatalf("offset = %d, want 4", p.ScrollOffset())
	}
	scrolled := p.Render()
	if scrolled == live {
		t.Fatal("scrolled render is identical to the live render")
	}
	if !strings.Contains(ansi.Strip(scrolled), "line4") {
		t.Fatalf("scrolled render missing the expected history: %q", ansi.Strip(scrolled))
	}

	p.ScrollDown(4)
	if p.ScrollOffset() != 0 {
		t.Fatalf("offset = %d, want 0", p.ScrollOffset())
	}
	if got := p.Render(); got != live {
		t.Fatalf("back at the live tail render = %q, want %q", got, live)
	}
}

func TestPaneScrollbackKeepsStyles(t *testing.T) {
	p := newTestPane(t)
	write(t, p, "\x1b[38;2;10;20;30mtagged\x1b[m\r\n")
	write(t, p, strings.Repeat("filler\r\n", 6))

	p.ScrollUp(p.term.ScrollbackLen())
	out := p.Render()
	if !strings.Contains(ansi.Strip(out), "tagged") {
		t.Fatalf("scrolled-back line missing from %q", ansi.Strip(out))
	}
	if !strings.Contains(out, "38;2;10;20;30") {
		t.Fatalf("scrolled-back line lost its style: %q", out)
	}
}

func TestPaneScrollbackLineTruncatedToPaneWidth(t *testing.T) {
	p := NewPane(PaneParams{Size: PaneSize{Cols: 40, Rows: 2}, ScrollbackSize: 10})
	write(t, p, strings.Repeat("x", 40)+"\r\n", "a\r\n", "b\r\n")

	p.Resize(PaneSize{Cols: 10, Rows: 2})
	p.ScrollUp(p.term.ScrollbackLen())
	for _, line := range strings.Split(p.Render(), "\n") {
		if got := ansi.StringWidth(line); got > 10 {
			t.Fatalf("line %q is %d wide, past the 10 columns of the pane", line, got)
		}
	}
}

func TestPaneScrollClampsBothEnds(t *testing.T) {
	p := newTestPane(t)
	write(t, p, strings.Repeat("line\r\n", 6))

	p.ScrollDown(10)
	if p.ScrollOffset() != 0 {
		t.Fatalf("offset = %d below the live tail, want 0", p.ScrollOffset())
	}

	p.ScrollUp(1000)
	if got, max := p.ScrollOffset(), p.term.ScrollbackLen(); got != max {
		t.Fatalf("offset = %d past the scrollback, want %d", got, max)
	}
}

func TestPaneScrollOffsetFollowsIncomingOutput(t *testing.T) {
	p := newTestPane(t)
	write(t, p, strings.Repeat("filler\r\n", 6))
	write(t, p, "anchor\r\n")
	write(t, p, strings.Repeat("tail\r\n", 2))

	p.ScrollUp(3)
	before := ansi.Strip(p.Render())
	if !strings.Contains(before, "anchor") {
		t.Fatalf("anchor line not in view: %q", before)
	}

	write(t, p, strings.Repeat("more\r\n", 2))
	if got := ansi.Strip(p.Render()); got != before {
		t.Fatalf("view drifted while scrolled back:\n got %q\nwant %q", got, before)
	}
}

// The buffer full is the regime the anchor is for: a job printing forever pushes
// a line out for every line in, and a scrollback length that no longer moves
// says nothing about how far the text slid.
func TestPaneScrollOffsetFollowsIncomingOutputOnAFullScrollback(t *testing.T) {
	p := newTestPane(t)
	for i := range 40 {
		write(t, p, fmt.Sprintf("line%02d\r\n", i))
	}
	if got := p.term.ScrollbackLen(); got != 10 {
		t.Fatalf("scrollback holds %d lines, want the 10 of a full buffer", got)
	}

	p.ScrollUp(4)
	before := ansi.Strip(p.Render())

	for i := range 4 {
		write(t, p, fmt.Sprintf("more%02d\r\n", i))
	}

	if got := ansi.Strip(p.Render()); got != before {
		t.Fatalf("view drifted while scrolled back on a full scrollback:\n got %q\nwant %q", got, before)
	}
}

// Past the room the pane holds for a reader, the lines being read are gone for
// good: the anchor stops at the oldest line left rather than sliding on.
func TestPaneScrollAnchorStopsAtTheOldestLineLeft(t *testing.T) {
	p := newTestPane(t)
	write(t, p, strings.Repeat("old\r\n", 12))

	p.ScrollUp(4)
	write(t, p, strings.Repeat("new\r\n", 40))

	if got, oldest := p.ScrollOffset(), p.term.ScrollbackLen(); got != oldest {
		t.Fatalf("offset = %d, want the %d of the oldest line left", got, oldest)
	}
	view := ansi.Strip(p.Render())
	if strings.Contains(view, "old") {
		t.Fatalf("the burst left content it had evicted in view: %q", view)
	}
	if !strings.HasPrefix(view, "new") {
		t.Fatalf("view = %q, want it anchored on the oldest line left", view)
	}
}

// The room a reader gets is lent, not kept: a pane that never returned it would
// cost twice its budget for the rest of the board.
func TestPaneReleasesTheHistoryRoomAtTheLiveTail(t *testing.T) {
	p := newTestPane(t)
	write(t, p, strings.Repeat("line\r\n", 12))

	p.ScrollUp(4)
	write(t, p, strings.Repeat("more\r\n", 20))
	if got := p.term.ScrollbackLen(); got <= 10 {
		t.Fatalf("scrollback holds %d lines, want the reader's history held past the budget", got)
	}

	p.ScrollToLive()
	if got := p.term.ScrollbackLen(); got != 10 {
		t.Fatalf("scrollback holds %d lines back at the live tail, want the 10 it budgets for", got)
	}
}

func TestPaneScrollToLive(t *testing.T) {
	p := newTestPane(t)
	write(t, p, strings.Repeat("line\r\n", 6))
	live := p.Render()

	p.ScrollUp(2)
	p.ScrollToLive()
	if got := p.Render(); got != live {
		t.Fatalf("ScrollToLive render = %q, want %q", got, live)
	}
}

func equalLines(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestPaneWriteAndRenderAreConcurrencySafe(t *testing.T) {
	p := newTestPane(t)
	// t.Fatalf outside the test goroutine is undefined, so the writer reports
	// back instead of failing where it runs.
	failed := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			if _, err := p.Write([]byte("\x1b[32mchunk\x1b[m\r\n")); err != nil {
				select {
				case failed <- err:
				default:
				}
				return
			}
		}
	}()
	for range 200 {
		p.ScrollUp(1)
		_ = p.Render()
		p.ScrollToLive()
	}
	<-done

	select {
	case err := <-failed:
		t.Fatalf("concurrent write: %v", err)
	default:
	}
}
