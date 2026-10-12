package render

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	unitStatusFmt = "%s · %s"
	phaseLabelFmt = "%s · %s"
	lineEnd       = "\n"
)

type ProgressViewParams struct {
	W     io.Writer
	Human bool
	// Live redraws in place: a terminal the view may repaint.
	Live bool
}

// ProgressView shows a run while it applies, on stderr, and keeps nothing of it
// but the line each hook phase leaves: the conclusion says what happened.
//
// It never takes the terminal into raw mode, so Ctrl+C reaches the process as a
// signal: the first cancels the run's context and the view says so, the second
// is the root's to handle.
type ProgressView struct {
	w       io.Writer
	human   bool
	live    bool
	mu      sync.Mutex
	unit    string
	status  string
	painted bool
	hook    *HookView
	phase   domain.HookBeat
	started time.Time
	stop    context.CancelFunc
	closed  bool
}

// NewProgressView watches ctx until Close: a cancelled run reads "Cancelling…"
// while it unwinds.
func NewProgressView(ctx context.Context, params ProgressViewParams) *ProgressView {
	watch, stop := context.WithCancel(ctx)
	view := &ProgressView{w: params.W, human: params.Human, live: params.Human && params.Live, stop: stop}
	if view.live {
		go view.cancelling(watch, ctx)
	}
	return view
}

func (v *ProgressView) cancelling(watch, run context.Context) {
	<-watch.Done()
	if run.Err() == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return
	}
	v.status = domain.CancellingMessage
	v.paint()
}

func (v *ProgressView) Emit(event kernel.Progress) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if event.Kind == kernel.ProgressOutput {
		v.output(event.Line)
		return
	}
	if !v.human {
		return
	}
	if event.Kind == kernel.ProgressPhaseStarted {
		v.phaseStarted(event)
		return
	}
	if !v.live {
		return
	}
	switch event.Kind {
	case kernel.ProgressPhaseFinished:
		v.phaseFinished()
	case kernel.ProgressUnitStarted:
		v.unit, v.status = event.Subject, ""
		v.paint()
	case kernel.ProgressStatus:
		v.status = text.Message(event.Code, event.Params)
		v.paint()
	case kernel.ProgressUnitFinished:
		v.unit, v.status = "", ""
		v.erase()
	}
}

// output passes a hook's line on as it came, unless a live phase tails it.
func (v *ProgressView) output(line string) {
	if v.hook != nil {
		_, _ = io.WriteString(v.hook, line+lineEnd)
		return
	}
	_, _ = io.WriteString(v.w, line+lineEnd)
}

func (v *ProgressView) phaseStarted(event kernel.Progress) {
	label := fmt.Sprintf(phaseLabelFmt, event.Params[kernel.ParamPhase], event.Subject)
	if !v.live {
		SectionTitle(v.w, label)
		return
	}
	v.erase()
	v.phase, v.started = domain.HookBeat{Cmd: label, Started: true}, time.Now()
	v.hook = NewHookView(HookViewParams{W: v.w})
	v.hook.OnHook(v.phase)
}

func (v *ProgressView) phaseFinished() {
	if v.hook == nil {
		return
	}
	v.hook.OnHook(domain.HookBeat{Cmd: v.phase.Cmd, Duration: time.Since(v.started)})
	v.hook = nil
	v.paint()
}

func (v *ProgressView) paint() {
	if !v.live || v.hook != nil || (v.unit == "" && v.status == "") {
		return
	}
	v.erase()
	Loading(v.w, styles.Truncate(styles.TruncateParams{Value: v.line(), Width: v.width()}))
	v.painted = true
}

// width leaves room for the indent and the glyph; a stream with no width of
// its own gets the hook view's.
func (v *ProgressView) width() int {
	cols := TerminalWidthOf(v.w)
	if cols <= 0 {
		return domain.HookViewFallbackWidth
	}
	return max(cols-2*len([]rune(Indent)), domain.HookViewMinWidth)
}

func (v *ProgressView) line() string {
	switch {
	case v.status == "":
		return v.unit
	case v.unit == "":
		return v.status
	default:
		return fmt.Sprintf(unitStatusFmt, v.unit, v.status)
	}
}

func (v *ProgressView) erase() {
	if !v.painted {
		return
	}
	fmt.Fprintf(v.w, domain.AnsiPrevLineFmt+domain.AnsiClearBelow, 1)
	v.painted = false
}

// Close erases what is still drawn; a phase an interruption cut short leaves no
// half-drawn tail above the conclusion.
func (v *ProgressView) Close() {
	v.stop()
	v.mu.Lock()
	defer v.mu.Unlock()
	v.closed = true
	if v.hook != nil {
		v.hook.Close()
		v.hook = nil
	}
	v.erase()
}
