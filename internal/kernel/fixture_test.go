package kernel_test

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// The tests share one example, shaped like `wtm create`: a request naming
// branches to create, the facts read from the repository, and a fake disk.

type isolation string

type target struct {
	From string `json:"from,omitempty"`
}

type request struct {
	Branches  []string          `json:"branches"`
	From      string            `json:"from,omitempty"`
	Isolation isolation         `json:"isolation,omitempty"`
	Push      bool              `json:"push,omitempty"`
	NoPush    bool              `json:"no_push,omitempty"`
	URLHost   string            `json:"url_host,omitempty"`
	URLPort   string            `json:"url_port,omitempty"`
	Decisions map[string]string `json:"decisions,omitempty"`
	Target    target            `json:"target"`
}

type facts struct {
	Existing []string
	Base     string
}

var errBoom = errors.New("boom")

func fail(context.Context) error { return errBoom }

// disk is what units write: the names that exist, in order.
type disk struct {
	mu      sync.Mutex
	entries []string
}

func (d *disk) add(name string) func(context.Context) error {
	return func(context.Context) error {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.entries = append(d.entries, name)
		return nil
	}
}

func (d *disk) remove(name string) func(context.Context) error {
	return func(context.Context) error {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.entries = slices.DeleteFunc(d.entries, func(entry string) bool { return entry == name })
		return nil
	}
}

func (d *disk) has(name string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Contains(d.entries, name)
}

func (d *disk) list() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.entries)
}

// recorder is an Emitter keeping every Progress it is handed.
type recorder struct {
	mu   sync.Mutex
	seen []kernel.Progress
}

func (r *recorder) Emit(progress kernel.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, progress)
}

// lines reads the progress back as "subject kind phase", one per event.
func (r *recorder) lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := make([]string, 0, len(r.seen))
	for _, progress := range r.seen {
		line := progress.Subject + " " + string(progress.Kind)
		if phase := progress.Params[kernel.ParamPhase]; phase != "" {
			line += " " + phase
		}
		if status := progress.Params[kernel.ParamStatus]; status != "" {
			line += " " + status
		}
		lines = append(lines, line)
	}
	return lines
}
