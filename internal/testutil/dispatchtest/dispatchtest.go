// Package dispatchtest runs a command through dispatch with no surface: a
// request in, the outcome, the progress and the events out. Its ports are
// doubles (a recorded bus, a fixed clock, numbered operations, a temporary
// state dir); the ports of a command are doubled beside the package that
// defines them.
package dispatchtest

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

// Clock is the time every operation of the bench starts at.
var Clock = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

// Bus records what dispatch publishes.
type Bus struct {
	mu     sync.Mutex
	events []domain.Event
}

func (b *Bus) Publish(_ context.Context, event domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
}

func (b *Bus) Events() []domain.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.events)
}

func (b *Bus) Types() []domain.EventType {
	types := []domain.EventType{}
	for _, event := range b.Events() {
		types = append(types, event.Type)
	}
	return types
}

// Progress collects what a command emits.
type Progress struct {
	mu   sync.Mutex
	seen []kernel.Progress
}

func (p *Progress) Emit(progress kernel.Progress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, progress)
}

func (p *Progress) Seen() []kernel.Progress {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.seen)
}

// Bench is a dispatcher on doubles; StateDir is where its locks and hook logs
// land.
type Bench struct {
	Dispatcher *dispatch.Dispatcher
	Bus        *Bus
	StateDir   string
	Dir        string
}

type BenchParams struct {
	// StateDir defaults to a temporary directory; another bench pointed at the
	// same one contends for the same locks.
	StateDir string
	// OpPrefix starts each operation id, "op-" by default: two benches on one
	// state dir need different ones, as two processes draw different ids.
	OpPrefix string
}

const defaultOpPrefix = "op-"

func New(t testing.TB, params BenchParams) *Bench {
	t.Helper()
	stateDir := params.StateDir
	if stateDir == "" {
		stateDir = t.TempDir()
	}
	prefix := params.OpPrefix
	if prefix == "" {
		prefix = defaultOpPrefix
	}
	bus := &Bus{}
	var mu sync.Mutex
	next := 0
	dir := t.TempDir()
	return &Bench{
		Bus:      bus,
		StateDir: stateDir,
		Dir:      dir,
		Dispatcher: dispatch.New(dispatch.Params{
			Dir:       dir,
			StateDir:  stateDir,
			Publisher: bus,
			Now:       func() time.Time { return Clock },
			NewOpID: func() kernel.OpID {
				mu.Lock()
				defer mu.Unlock()
				next++
				return kernel.OpID(fmt.Sprintf("%s%d", prefix, next))
			},
		}),
	}
}

type RunParams[Req, F, P, D any] struct {
	Bench   *Bench
	Command kernel.Command[Req, F, P, D]
	Request Req
	Options dispatch.Options
}

// Ran is what a run returned and what it emitted.
type Ran[P, D any] struct {
	Result   dispatch.Result[P, D]
	Err      error
	Progress []kernel.Progress
}

// Run runs a command of the bench the way a surface would, without one.
func Run[Req, F, P, D any](ctx context.Context, params RunParams[Req, F, P, D]) Ran[P, D] {
	handle := dispatch.Register(dispatch.RegisterParams[Req, F, P, D]{Dispatcher: params.Bench.Dispatcher, Command: params.Command})
	progress := &Progress{}
	result, err := handle.Run(ctx, dispatch.RunParams[Req]{Request: params.Request, Options: params.Options, Progress: progress})
	return Ran[P, D]{Result: result, Err: err, Progress: progress.Seen()}
}
