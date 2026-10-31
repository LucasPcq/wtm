package output

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// concurrentTick is how often a live progress redraws the time its rows have run.
const concurrentTick = time.Second

// Rows says what a [Concurrent] progress draws a row of.
type Rows int

// The rows of a [Concurrent] progress.
const (
	RowPerUnit Rows = iota // a row per subject, from its unit's start to its end
	RowPerJob              // a row per job a subject starts, from its phase's start to its end
)

// ConcurrentParams are where a [Concurrent] progress draws and how, and what it draws a row of.
type ConcurrentParams struct {
	ProgressParams
	Rows Rows
	Tick time.Duration // how often a live row redraws its time; a second when zero
}

// A Concurrent progress draws a run whose subjects run at once: a line per row as it ends, with how long it ran, and, live, a row per one still running beneath them, redrawn in place; once the run is cancelled, what it is still finishing.
// Its methods may be called from several goroutines.
type Concurrent struct {
	w       io.Writer
	human   bool
	live    bool
	rows    Rows
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

// NewConcurrent returns the progress of the run ctx runs; for a person it announces the cancellation as soon as ctx is cancelled, until Close.
func NewConcurrent(ctx context.Context, params ConcurrentParams) *Concurrent {
	tick := params.Tick
	if tick == 0 {
		tick = concurrentTick
	}
	p := &Concurrent{w: params.W, human: params.Human, live: params.Human && params.Live, rows: params.Rows, started: map[string]time.Time{}, run: ctx, tick: tick, done: make(chan struct{})}

	if params.Human {
		go p.watch()
	}
	return p
}

// watch announces the cancel, then, live, keeps redrawing the time each target runs until Close.
func (p *Concurrent) watch() {
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

func (p *Concurrent) locked(do func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	do()
}

// announce tells, once, that the run is cancelling and which targets it is still finishing.
func (p *Concurrent) announce() {
	if p.closed || p.notice != "" {
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

// Report draws event: it is the domain.Report the command hands its use case.
func (p *Concurrent) Report(event domain.Event) {
	if !p.human {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	switch e := event.(type) {
	case domain.UnitStarted:
		if p.rows == RowPerUnit {
			p.begin(e.Subject)
		}
	case domain.UnitFinished:
		switch {
		case p.rows == RowPerUnit:
			p.end(e.Subject, e.Status)
		case e.Status == domain.StatusCancelled || e.Status == domain.StatusSkipped:
			WarningLine(p.w, e.Subject+" interrupted")
		}
	case domain.PhaseStarted:
		if p.rows == RowPerJob && e.Job != "" {
			p.begin(jobRow(e.Subject, e.Job))
		}
	case domain.PhaseFinished:
		if p.rows == RowPerJob && e.Job != "" {
			p.end(jobRow(e.Subject, e.Job), statusOf(e.Failed))
		}
	case domain.Output, domain.WorktreeCreated, domain.WorktreeUpdated, domain.WorktreeChanged, domain.WorktreeProvisioned, domain.WorktreeRemoved, domain.OperationStarted, domain.OperationFinished, domain.Snapshot, domain.Ready, domain.RepoAdded, domain.RepoRemoved, domain.JobStarted, domain.JobStopped, domain.JobCrashed, domain.JobExited:
	}
}

func jobRow(subject, job string) string {
	return subject + Dot + job
}

func statusOf(failed bool) domain.Status {
	if failed {
		return domain.StatusFailed
	}
	return domain.StatusDone
}

// begin adds the row of key, running.
func (p *Concurrent) begin(key string) {
	p.running, p.started[key] = append(p.running, key), time.Now()
	p.paint()
}

// end writes the line of the row of key, which ended with status.
func (p *Concurrent) end(key string, status domain.Status) {
	p.erase()
	p.running = slices.DeleteFunc(p.running, func(running string) bool { return running == key })
	p.finished(key, status)
	p.paint()
}

// finished writes the line of a row that ended: done, failed, or stopped.
func (p *Concurrent) finished(key string, status domain.Status) {
	line := key + " (" + Duration(time.Since(p.started[key])) + ")"
	switch status {
	case domain.StatusDone:
		SuccessLine(p.w, line)
	case domain.StatusFailed:
		ErrorLine(p.w, line)
	case domain.StatusCancelled, domain.StatusSkipped:
		WarningLine(p.w, key+" interrupted")
	}
}

// paint redraws, live, the notice of a cancel and a row per target running, with how long it has run.
func (p *Concurrent) paint() {
	if !p.live || p.closed {
		return
	}

	p.erase()
	width := BlockWidth(p.w)
	if p.notice != "" {
		Message(p.w, Truncate(p.notice, width))
		p.painted++
	}
	for _, subject := range p.running {
		row := subject
		if ran := time.Since(p.started[subject]).Truncate(time.Second); ran > 0 {
			row += Dot + Duration(ran)
		}
		LoadingLine(p.w, Truncate(row, width))
		p.painted++
	}
}

func (p *Concurrent) erase() {
	EraseLines(p.w, p.painted)
	p.painted = 0
}

// Close erases what the progress still draws and stops it announcing a cancellation; the command's output follows on a clean screen.
func (p *Concurrent) Close() {
	close(p.done)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.erase()
	p.closed = true
}
