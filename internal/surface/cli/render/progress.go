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

var failedBeat = map[bool]string{true: "failed"}

const (
	unitStatusFmt = "%s · %s"
	phaseLabelFmt = "%s · %s"
	lineEnd       = "\n"
)

type ProgressViewParams struct {
	W       io.Writer
	Human   bool
	Catalog text.Catalog
	Live    bool
}

type ProgressView struct {
	w         io.Writer
	human     bool
	catalog   text.Catalog
	live      bool
	mu        sync.Mutex
	unit      string
	status    string
	painted   bool
	hook      *HookView
	phase     string
	phaseLog  string
	phaseOpen bool
	started   time.Time
	stop      context.CancelFunc
	closed    bool
}

func NewProgressView(ctx context.Context, params ProgressViewParams) *ProgressView {
	watch, stop := context.WithCancel(ctx)
	view := &ProgressView{w: params.W, human: params.Human, catalog: params.Catalog, live: params.Human && params.Live, stop: stop}
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
	if output, ok := event.(*kernel.Output); ok {
		v.output(output.Line)
		return
	}
	if !v.human {
		return
	}
	switch e := event.(type) {
	case *kernel.PhaseStarted:
		v.phase, v.phaseLog, v.phaseOpen, v.started = fmt.Sprintf(phaseLabelFmt, e.Phase, e.Subject), e.Log, false, time.Now()
	case *kernel.PhaseFinished:
		v.phaseFinished(e.Failed)
	case *kernel.UnitStarted:
		v.unit, v.status = e.Subject, ""
		v.paint()
	case *kernel.StatusChanged:
		v.status = v.catalog.Message(e.Code, e.Params)
		v.paint()
	case *kernel.UnitFinished:
		v.unit, v.status = "", ""
		v.erase()
	case *kernel.Output:
	}
}

func (v *ProgressView) output(line string) {
	if v.human && v.phase != "" && !v.phaseOpen {
		v.openPhase()
	}
	if v.hook != nil {
		_, _ = io.WriteString(v.hook, line+lineEnd)
		return
	}
	_, _ = io.WriteString(v.w, line+lineEnd)
}

func (v *ProgressView) openPhase() {
	v.phaseOpen = true
	if !v.live {
		SectionTitle(v.w, v.phase)
		return
	}
	v.erase()
	v.hook = NewHookView(HookViewParams{W: v.w, LogPath: v.phaseLog})
	v.hook.OnHook(domain.HookBeat{Cmd: v.phase, Started: true})
}

func (v *ProgressView) phaseFinished(failed bool) {
	label := v.phase
	v.phase, v.phaseLog, v.phaseOpen = "", "", false
	if v.hook == nil {
		return
	}
	v.hook.OnHook(domain.HookBeat{Cmd: label, Duration: time.Since(v.started), Err: failedBeat[failed]})
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
