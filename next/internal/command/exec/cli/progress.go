package cli

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// progressTick is how often a live progress redraws the time its targets have run.
const progressTick = time.Second

// A progress draws a run whose targets run at once: a line per target as it ends, with how long it ran, and, live, a row per target still running beneath them, redrawn in place; once the run is cancelled, what it is still finishing.
// Its methods may be called from several goroutines.
type progress struct {
	w       io.Writer
	human   bool
	live    bool
	mu      sync.Mutex
	running []string
	started map[string]time.Time
	painted int
	run     context.Context
	tick    time.Duration
	done    chan struct{}
	notice  string
	closed  bool
}

// newProgress returns the progress of the run ctx runs; for a person it announces the cancellation as soon as ctx is cancelled, until Close.
func newProgress(ctx context.Context, params output.ProgressParams) *progress {
	return newProgressEvery(ctx, params, progressTick)
}

// newProgressEvery is newProgress redrawn, live, every tick.
func newProgressEvery(ctx context.Context, params output.ProgressParams, tick time.Duration) *progress {
	p := &progress{w: params.W, human: params.Human, live: params.Human && params.Live, started: map[string]time.Time{}, run: ctx, tick: tick, done: make(chan struct{})}

	if params.Human {
		go p.watch()
	}
	return p
}

// watch announces the cancel, then, live, keeps redrawing the time each target runs until Close.
func (p *progress) watch() {
	ticks := make(<-chan time.Time)
	if p.live {
		ticker := time.NewTicker(p.tick)
		defer ticker.Stop()
		ticks = ticker.C
	}

	cancelled := p.run.Done()
	for {
		select {
		case <-cancelled:
			cancelled = nil
			p.locked(p.announce)
		case <-ticks:
			p.locked(p.paint)
		case <-p.done:
			return
		}
	}
}

func (p *progress) locked(do func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	do()
}

// announce tells, once, that the run is cancelling and which targets it is still finishing.
func (p *progress) announce() {
	if p.closed || p.notice != "" {
		return
	}

	p.notice = "Cancelling — Ctrl+C again to stop now"
	if len(p.running) > 0 {
		p.notice = "Cancelling — finishing " + strings.Join(p.running, ", ") + "; Ctrl+C again to stop now"
	}
	if !p.live {
		output.Message(p.w, p.notice)
		return
	}
	p.paint()
}

// Report draws event: it is the domain.Report the command hands its use case.
func (p *progress) Report(event domain.Event) {
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

// finished writes the line of a target that ended: done, failed, or stopped.
func (p *progress) finished(e domain.UnitFinished) {
	line := e.Subject + " (" + output.Duration(time.Since(p.started[e.Subject])) + ")"
	switch e.Status {
	case domain.StatusDone:
		output.SuccessLine(p.w, line)
	case domain.StatusFailed:
		output.ErrorLine(p.w, line)
	case domain.StatusCancelled, domain.StatusSkipped:
		output.WarningLine(p.w, e.Subject+" interrupted")
	}
}

// paint redraws, live, the notice of a cancel and a row per target running, with how long it has run.
func (p *progress) paint() {
	if !p.live || p.closed {
		return
	}

	p.erase()
	width := output.BlockWidth(p.w)
	if p.notice != "" {
		output.Message(p.w, output.Truncate(p.notice, width))
		p.painted++
	}
	for _, subject := range p.running {
		row := subject
		if ran := time.Since(p.started[subject]).Truncate(time.Second); ran > 0 {
			row += output.Dot + output.Duration(ran)
		}
		output.LoadingLine(p.w, output.Truncate(row, width))
		p.painted++
	}
}

func (p *progress) erase() {
	output.EraseLines(p.w, p.painted)
	p.painted = 0
}

// Close erases what the progress still draws and stops it announcing a cancellation; the command's output follows on a clean screen.
func (p *progress) Close() {
	close(p.done)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.erase()
	p.closed = true
}
