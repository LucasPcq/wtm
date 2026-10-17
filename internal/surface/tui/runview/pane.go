// Package runview renders a job's raw PTY output through a terminal emulator.
package runview

import (
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type PaneSize struct {
	Cols int
	Rows int
}

type PaneParams struct {
	Size           PaneSize
	ScrollbackSize int
}

// Pane serializes access to the emulator itself: vt.SafeEmulator guards only
// part of what it exposes — WriteString, String, Close, Bounds and InputPipe
// are promoted unlocked from the embedded *Emulator.
type Pane struct {
	mu         sync.Mutex
	term       *vt.Emulator
	size       PaneSize
	scrollback int
	offset     int
	dirty      bool
}

func NewPane(params PaneParams) *Pane {
	size := resolvePaneSize(params.Size)
	pane := &Pane{
		term:       vt.NewEmulator(size.Cols, size.Rows),
		size:       size,
		scrollback: orDefault(params.ScrollbackSize, domain.JobPaneScrollbackLines),
	}
	pane.term.SetScrollbackSize(pane.scrollback)
	return pane
}

func (p *Pane) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	held := p.term.ScrollbackLen()
	n, err := p.term.Write(b)
	p.followLocked(held)
	p.dirty = true
	return n, err
}

// TakeDirty raises on writes alone: a scroll, a resize or a selection reaches
// the screen on the message that caused it, which Bubbletea redraws after.
func (p *Pane) TakeDirty() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	written := p.dirty
	p.dirty = false
	return written
}

// Resize reports whether the size actually changed. x/vt never reflows, so only
// the job's own process can redraw at the new size and the caller has to resize
// its PTY too — a round-trip worth skipping when nothing moved.
func (p *Pane) Resize(size PaneSize) (changed bool) {
	next := resolvePaneSize(size)
	p.mu.Lock()
	defer p.mu.Unlock()
	if next == p.size {
		return false
	}
	p.scrollIntoHistoryLocked(next.Rows)
	p.term.Resize(next.Cols, next.Rows)
	p.size = next
	p.clampLocked()
	return true
}

// scrollIntoHistoryLocked makes room for a shorter screen the way a terminal
// does. x/vt drops the bottom rows on a shrink and never gives them back, so
// the rows under the new last one are pushed up into the scrollback first —
// otherwise the newest output goes, which on the frame the abort report just
// shortened is the crash the reader opened the view for.
func (p *Pane) scrollIntoHistoryLocked(rows int) {
	lost := p.term.CursorPosition().Y - (rows - 1)
	if lost <= 0 {
		return
	}
	held := p.term.ScrollbackLen()
	p.term.Write([]byte(ansi.ScrollUp(lost)))
	p.followLocked(held)
}

func (p *Pane) Size() PaneSize {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.size
}

func (p *Pane) ScrollUp(lines int) { p.scrollBy(lines) }

func (p *Pane) ScrollDown(lines int) { p.scrollBy(-lines) }

func (p *Pane) ScrollToLive() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset = 0
	p.holdHistoryLocked()
}

// ScrollOffset is the number of scrollback lines above the live tail; zero
// means the pane follows the job's output.
func (p *Pane) ScrollOffset() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.offset
}

// Render returns the visible rows with their SGR sequences, ready to drop into
// a lipgloss view.
func (p *Pane) Render() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.offset == 0 {
		return p.term.Render()
	}

	scrollback := p.term.Scrollback()
	lines := make([]string, 0, p.size.Rows)
	for i := p.offset; i > 0 && len(lines) < p.size.Rows; i-- {
		line := scrollback.Line(scrollback.Len() - i).Render()
		lines = append(lines, ansi.Truncate(line, p.size.Cols, ""))
	}
	if len(lines) == p.size.Rows {
		return strings.Join(lines, "\n")
	}

	for line := range strings.SplitSeq(p.term.Render(), "\n") {
		if len(lines) == p.size.Rows {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (p *Pane) scrollBy(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offset += lines
	p.clampLocked()
	p.holdHistoryLocked()
}

// followLocked keeps a scrolled-back reader on the same lines by moving the
// anchor as far as the write pushed the history up. That distance is how much
// the scrollback grew — but only while nothing is being evicted: a full buffer
// keeps its length as it slides, and an anchor following that would follow the
// tail instead of the text. holdHistoryLocked is what buys the room, and the
// pane runs out of it only when a single write pushed more lines than the whole
// history holds, at which point nothing the reader anchored on is left and the
// anchor drops to the oldest line still held.
func (p *Pane) followLocked(held int) {
	if p.offset == 0 {
		return
	}
	if p.term.ScrollbackLen() >= p.historyCapacity() {
		p.offset = p.term.ScrollbackLen()
		return
	}
	p.offset += p.term.ScrollbackLen() - held
	p.clampLocked()
}

// holdHistoryLocked widens the emulator's buffer past what the pane retains for
// as long as someone is reading back through it, and gives the room back — the
// oldest lines with it — as soon as the pane is following the job again.
func (p *Pane) holdHistoryLocked() {
	if p.offset > 0 {
		p.term.SetScrollbackSize(p.historyCapacity())
		return
	}
	p.term.SetScrollbackSize(p.scrollback)
}

func (p *Pane) historyCapacity() int {
	return p.scrollback * domain.JobPaneScrollbackBurstFactor
}

func (p *Pane) clampLocked() {
	p.offset = min(max(p.offset, 0), p.term.ScrollbackLen())
}

func resolvePaneSize(size PaneSize) PaneSize {
	return PaneSize{
		Cols: orDefault(size.Cols, domain.JobPaneDefaultCols),
		Rows: orDefault(size.Rows, domain.JobPaneDefaultRows),
	}
}

func orDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
