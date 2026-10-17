package output

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ProgressParams is where a [Progress] draws and how.
type ProgressParams struct {
	W io.Writer
	// Human reports whether the output is for a person; without it only the hooks' output reaches W.
	Human bool
	// Live reports whether the progress may redraw itself on W, a terminal; it has no effect without Human.
	Live bool
}

// A Progress draws the events of a running operation: the unit in progress on a line it redraws, the output of a hook under its phase, and, once the operation is cancelled, what it is still finishing.
// Its methods may be called from several goroutines.
type Progress struct {
	w         io.Writer
	human     bool
	live      bool
	mu        sync.Mutex
	unit      string
	status    string
	painted   bool
	hook      *hookView
	phase     string
	phaseLog  string
	phaseOpen bool
	started   time.Time
	run       context.Context
	stop      context.CancelFunc
	announced bool
	closed    bool
}

// NewProgress returns the progress of the operation ctx runs; for a person it announces the cancellation as soon as ctx is cancelled, until [Progress.Close].
func NewProgress(ctx context.Context, params ProgressParams) *Progress {
	watch, stop := context.WithCancel(ctx)
	progress := &Progress{w: params.W, human: params.Human, live: params.Human && params.Live, run: ctx, stop: stop}

	if params.Human {
		go progress.cancelling(watch)
	}
	return progress
}

func (p *Progress) cancelling(watch context.Context) {
	<-watch.Done()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.announce()
}

// announce tells, once, that the operation is cancelling and what it is still finishing; it does nothing until the operation's context is cancelled.
func (p *Progress) announce() {
	if p.closed || p.announced || p.run.Err() == nil {
		return
	}

	p.announced = true
	p.status = p.cancellingMessage()
	if !p.live {
		Message(p.w, p.status)
		return
	}
	if p.hook != nil {
		p.hook.notify(p.status)
		return
	}
	p.paint()
}

func (p *Progress) cancellingMessage() string {
	switch {
	case p.phase != "" && p.unit != "":
		return "Cancelling — finishing " + p.phase + " of " + p.unit + "; Ctrl+C again to stop now"
	case p.unit != "":
		return "Cancelling — finishing " + p.unit + "; Ctrl+C again to stop now"
	}
	return "Cancelling — Ctrl+C again to stop now"
}

// Report draws event: it is the [domain.Report] a command hands its use case.
func (p *Progress) Report(event domain.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.human {
		p.announce()
	}

	if output, ok := event.(domain.Output); ok {
		p.output(output.Line)
		return
	}
	if !p.human {
		return
	}

	switch e := event.(type) {
	case domain.PhaseStarted:
		p.phase, p.phaseLog, p.phaseOpen, p.started = e.Phase, e.Log, false, time.Now()
	case domain.PhaseFinished:
		p.phaseFinished(e.Failed)
	case domain.UnitStarted:
		p.unit, p.status = e.Subject, ""
		p.paint()
	case domain.UnitFinished:
		p.unit, p.status = "", ""
		p.erase()
	case domain.Output, domain.WorktreeCreated, domain.WorktreeNumbered, domain.WorktreeProvisioned, domain.OperationStarted, domain.OperationFinished:
	}
}

func (p *Progress) label() string {
	return p.phase + Dot + p.unit
}

// output writes a line a hook printed, opening its phase first: a phase that prints nothing shows nothing.
func (p *Progress) output(line string) {
	if p.human && p.phase != "" && !p.phaseOpen {
		p.openPhase()
	}

	if p.hook != nil {
		_, _ = io.WriteString(p.hook, line+"\n")
		return
	}
	_, _ = io.WriteString(p.w, line+"\n")
}

// openPhase starts drawing the phase in progress: a title above its output, or live, a view of its last lines in place of the status line.
func (p *Progress) openPhase() {
	p.phaseOpen = true
	if !p.live {
		SectionTitle(p.w, p.label())
		return
	}

	p.erase()
	p.hook = newHookView(hookViewParams{W: p.w, Label: p.label(), LogPath: p.phaseLog})
	if p.status != "" {
		p.hook.notify(p.status)
	}
}

// phaseFinished closes the phase in progress, replacing its live view with one line of its result, and brings back the status line.
func (p *Progress) phaseFinished(failed bool) {
	label := p.label()
	p.phase, p.phaseLog, p.phaseOpen = "", "", false
	if p.hook == nil {
		return
	}
	p.hook.finish(label, time.Since(p.started), failed)
	p.hook = nil

	p.paint()
}

// paint redraws the status line, live and when no phase's view stands in its place.
func (p *Progress) paint() {
	if !p.live || p.hook != nil || (p.unit == "" && p.status == "") {
		return
	}

	p.erase()
	loadingLine(p.w, truncate(p.line(), blockWidth(p.w)))
	p.painted = true
}

func (p *Progress) line() string {
	if p.status == "" {
		return p.unit
	}
	return p.status
}

func (p *Progress) erase() {
	if !p.painted {
		return
	}
	fmt.Fprintf(p.w, ansiPrevLine+ansiClearLine, 1)
	p.painted = false
}

// Close erases what the progress still draws and stops it announcing a cancellation; the command's output follows on a clean screen.
func (p *Progress) Close() {
	p.stop()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true
	if p.hook != nil {
		p.hook.close()
		p.hook = nil
	}

	p.erase()
}
