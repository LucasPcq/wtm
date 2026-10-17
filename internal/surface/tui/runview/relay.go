package runview

import (
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// relay is where a run reports, before and after the reader leaves. Leaving the
// view detaches from a run rather than aborting it, so the sequence keeps going
// with nobody watching it and its events have to land somewhere else.
//
// The window between the reader leaving and the surface taking over is the
// terminal being given back, which is long enough for a job to start in. Those
// events are held rather than dropped — a job that started while the screen was
// being restored is exactly what the reader left to let happen.
type relay struct {
	mu       sync.Mutex
	view     runlogs.Sink
	detached bool
	target   runlogs.Sink
	held     []runlogs.Event
	// output counts the held chunks alone. Lifecycle events are one per job and
	// bounded by the profile; a job's output is not bounded by anything, and the
	// hand-over is measured in milliseconds — past the cap it is the run that
	// matters, not the bytes nobody was there to read.
	output int
}

func newRelay(view runlogs.Sink) *relay { return &relay{view: view} }

func (r *relay) Emit(event runlogs.Event) {
	r.mu.Lock()
	switch {
	case !r.detached:
		view := r.view
		r.mu.Unlock()
		view.Emit(event)
	case r.target != nil:
		target := r.target
		r.mu.Unlock()
		target.Emit(event)
	default:
		r.hold(event)
		r.mu.Unlock()
	}
}

func (r *relay) hold(event runlogs.Event) {
	if event.Phase == runlogs.PhaseOutput {
		if r.output >= domain.RunDetachHeldChunks {
			return
		}
		r.output++
	}
	r.held = append(r.held, event)
}

// detach stops the view from being written to. It does not name a successor:
// the surface installs one once it has its terminal back.
func (r *relay) detach() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detached = true
}

// redirect hands the run to its new reporter, oldest held event first.
func (r *relay) redirect(target runlogs.Sink) {
	r.mu.Lock()
	held := r.held
	r.held, r.output, r.target, r.detached = nil, 0, target, true
	r.mu.Unlock()

	for _, event := range held {
		target.Emit(event)
	}
}
