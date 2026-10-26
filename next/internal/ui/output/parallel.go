package output

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// parallelTick is how often a live Parallel redraws the time its units have run.
const parallelTick = time.Second

// A Parallel draws the events of an operation whose units run at once: a line per unit as it ends, with how long it ran, and, live, a row per unit still running beneath them, redrawn in place; once the operation is cancelled, what it is still finishing.
// Its methods may be called from several goroutines.
type Parallel struct {
	w       io.Writer
	human   bool
	live    bool
	mu      sync.Mutex
	running []string
	started map[string]time.Time
	painted int
	run     context.Context
	stop    context.CancelFunc
	notice  string
	closed  bool
}

// NewParallel returns the progress of the operation ctx runs, its units at once; for a person it announces the cancellation as soon as ctx is cancelled, until [Parallel.Close].
func NewParallel(ctx context.Context, params ProgressParams) *Parallel {
	watch, stop := context.WithCancel(ctx)
	p := &Parallel{w: params.W, human: params.Human, live: params.Human && params.Live, started: map[string]time.Time{}, run: ctx, stop: stop}

	if params.Human {
		go p.watch(watch)
	}
	return p
}

func (p *Parallel) watch(watch context.Context) {
	ticks := make(<-chan time.Time)
	if p.live {
		ticker := time.NewTicker(parallelTick)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-watch.Done():
			p.mu.Lock()
			defer p.mu.Unlock()
			p.announce()
			return
		case <-ticks:
			p.mu.Lock()
			p.paint()
			p.mu.Unlock()
		}
	}
}

// announce tells, once, that the operation is cancelling and which units it is still finishing; it does nothing until the operation's context is cancelled.
func (p *Parallel) announce() {
	if p.closed || p.notice != "" || p.run.Err() == nil {
		return
	}

	p.notice = "Cancelling — Ctrl+C again to stop now"
	if len(p.running) > 0 {
		p.notice = "Cancelling — finishing " + strings.Join(p.running, ", ") + "; Ctrl+C again to stop now"
	}
	if !p.live {
		Message(p.w, p.notice)
		return
	}
	p.paint()
}

// Report draws event: it is the [domain.Report] a command hands its use case.
func (p *Parallel) Report(event domain.Event) {
	if !p.human {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	switch e := event.(type) {
	case domain.UnitStarted:
		p.running, p.started[e.Subject] = append(p.running, e.Subject), time.Now()
		p.paint()
	case domain.UnitFinished:
		p.erase()
		p.running = slices.DeleteFunc(p.running, func(subject string) bool { return subject == e.Subject })
		p.finished(e)
		p.paint()
	case domain.Output, domain.PhaseStarted, domain.PhaseFinished, domain.WorktreeCreated, domain.WorktreeUpdated, domain.WorktreeChanged, domain.WorktreeProvisioned, domain.WorktreeRemoved, domain.OperationStarted, domain.OperationFinished, domain.Snapshot, domain.Ready, domain.RepoAdded, domain.RepoRemoved:
	}
}

// finished writes the line of a unit that ended: done, failed, or stopped.
func (p *Parallel) finished(e domain.UnitFinished) {
	line := e.Subject + " (" + duration(time.Since(p.started[e.Subject])) + ")"
	switch e.Status {
	case domain.StatusDone:
		SuccessLine(p.w, line)
	case domain.StatusFailed:
		ErrorLine(p.w, line)
	case domain.StatusCancelled, domain.StatusSkipped:
		WarningLine(p.w, e.Subject+" interrupted")
	}
}

// paint redraws, live, the notice of a cancel and a row per unit running, with how long it has run.
func (p *Parallel) paint() {
	if !p.live || p.closed {
		return
	}

	p.erase()
	width := blockWidth(p.w)
	if p.notice != "" {
		Message(p.w, truncate(p.notice, width))
		p.painted++
	}
	for _, subject := range p.running {
		row := subject
		if ran := time.Since(p.started[subject]).Truncate(time.Second); ran > 0 {
			row += Dot + duration(ran)
		}
		loadingLine(p.w, truncate(row, width))
		p.painted++
	}
}

func (p *Parallel) erase() {
	if p.painted == 0 {
		return
	}
	fmt.Fprintf(p.w, ansiPrevLine+ansiClearBelow, p.painted)
	p.painted = 0
}

// Close erases what the progress still draws and stops it announcing a cancellation; the command's output follows on a clean screen.
func (p *Parallel) Close() {
	p.stop()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.erase()
	p.closed = true
}
