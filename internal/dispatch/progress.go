package dispatch

import (
	"io"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
)

const lineEnd = "\n"

var hookPhases = []domain.HookEvent{domain.HookOnCreate, domain.HookOnClean}

type progressParams struct {
	Bus     operationBus
	Forward kernel.Emitter
}

// progress is the Emitter Apply is handed: it stamps each event with the
// operation, publishes the end of each unit, keeps the output of each hook
// phase in its log, then forwards the event to the surface.
type progress struct {
	bus      operationBus
	forward  kernel.Emitter
	stateDir string
	mu       sync.Mutex
	logs     map[string]io.WriteCloser
}

func (d *Dispatcher) progress(params progressParams) *progress {
	return &progress{bus: params.Bus, forward: params.Forward, stateDir: d.params.StateDir, logs: map[string]io.WriteCloser{}}
}

func (p *progress) Emit(event kernel.Progress) {
	event.Op = p.bus.operation.ID
	p.log(event)
	if event.Kind == kernel.ProgressUnitFinished {
		p.bus.unitFinished(event)
	}
	if p.forward != nil {
		p.forward.Emit(event)
	}
}

// log writes a hook phase's whole output to the file `wtm` points a reader
// at, whatever the surface chose to show of it. A log that cannot be opened
// is skipped: it never stops the hook.
func (p *progress) log(event kernel.Progress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	phase := domain.HookEvent(event.Params[kernel.ParamPhase])
	switch event.Kind {
	case kernel.ProgressPhaseStarted:
		if p.stateDir == "" || !slices.Contains(hookPhases, phase) {
			return
		}
		file, err := fs.OpenLog(rules.HooksLogPath(rules.HooksLogPathParams{StateDir: p.stateDir, Phase: phase, Branch: event.Subject}))
		if err != nil {
			return
		}
		p.logs[event.Subject] = file
	case kernel.ProgressOutput:
		if file, open := p.logs[event.Subject]; open {
			_, _ = io.WriteString(file, event.Line+lineEnd)
		}
	case kernel.ProgressPhaseFinished:
		if slices.Contains(hookPhases, phase) {
			p.closeLog(event.Subject)
		}
	}
}

func (p *progress) closeLog(subject string) {
	if file, open := p.logs[subject]; open {
		_ = file.Close()
		delete(p.logs, subject)
	}
}

// close closes the logs of the phases an interruption left open.
func (p *progress) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for subject := range p.logs {
		p.closeLog(subject)
	}
}
