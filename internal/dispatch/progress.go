package dispatch

import (
	"io"
	"sync"

	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/kernel"
)

const lineEnd = "\n"

type progress struct {
	bus     operationBus
	forward kernel.Emitter
	mu      sync.Mutex
	logs    map[string]io.WriteCloser
}

type progressParams struct {
	Bus     operationBus
	Forward kernel.Emitter
}

func newProgress(params progressParams) *progress {
	return &progress{bus: params.Bus, forward: params.Forward, logs: map[string]io.WriteCloser{}}
}

func (p *progress) Emit(event kernel.Progress) {
	event.Base().Op = kernel.OpID(p.bus.operation.ID)
	p.log(event)
	if finished, ok := event.(*kernel.UnitFinished); ok {
		p.bus.unitFinished(finished)
	}
	if p.forward != nil {
		p.forward.Emit(event)
	}
}

// A log that cannot be opened never stops its phase.
func (p *progress) log(event kernel.Progress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch e := event.(type) {
	case *kernel.PhaseStarted:
		if e.Log == "" {
			return
		}
		file, err := fs.OpenLog(e.Log)
		if err != nil {
			return
		}
		p.logs[e.Subject] = file
	case *kernel.Output:
		if file, open := p.logs[e.Subject]; open {
			_, _ = io.WriteString(file, e.Line+lineEnd)
		}
	case *kernel.PhaseFinished:
		p.closeLog(e.Subject)
	case *kernel.UnitStarted, *kernel.UnitFinished, *kernel.StatusChanged:
	}
}

func (p *progress) closeLog(subject string) {
	if file, open := p.logs[subject]; open {
		_ = file.Close()
		delete(p.logs, subject)
	}
}

func (p *progress) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for subject := range p.logs {
		p.closeLog(subject)
	}
}
