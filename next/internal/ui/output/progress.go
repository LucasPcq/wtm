package output

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

type ProgressParams struct {
	W     io.Writer
	Human bool
	Live  bool
}

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

func (p *Progress) phaseFinished(failed bool) {
	label := p.label()
	p.phase, p.phaseLog, p.phaseOpen = "", "", false
	if p.hook == nil {
		return
	}
	p.hook.finish(hookEnd{Label: label, Duration: time.Since(p.started), Failed: failed})
	p.hook = nil
	p.paint()
}

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
