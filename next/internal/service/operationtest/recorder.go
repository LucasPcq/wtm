package operationtest

import (
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// A Recorder records the events a run reports, in order, safe across
// goroutines; On, when set before the run, is called with each one as it
// arrives. Its Open makes it the Publisher of an operations service.
type Recorder struct {
	On func(domain.Event)

	mu     sync.Mutex
	events []domain.Event
}

// Add records event: it is a domain.Report.
func (r *Recorder) Add(event domain.Event) {
	r.mu.Lock()
	r.events = append(r.events, event)
	on := r.On
	r.mu.Unlock()
	if on != nil {
		on(event)
	}
}

// Open returns Add, the report of every operation.
func (r *Recorder) Open(domain.Operation) domain.Report {
	return r.Add
}

// All returns the events recorded so far, in order.
func (r *Recorder) All() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}
