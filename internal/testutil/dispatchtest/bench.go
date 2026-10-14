package dispatchtest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
)

var Clock = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

type Bench struct {
	Dispatcher *dispatch.Dispatcher
	Bus        *Bus
	StateDir   string
	Dir        string
}

type BenchParams struct {
	StateDir string
	OpPrefix string // tells apart two benches sharing a state dir, as two processes draw different ids
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
	Events  dispatch.Events[F, D]
	Request Req
	Options dispatch.Options
}

type Ran[P, D any] struct {
	Result   dispatch.Result[P, D]
	Err      error
	Progress []kernel.Progress
}

func Run[Req, F, P, D any](ctx context.Context, params RunParams[Req, F, P, D]) Ran[P, D] {
	handle := dispatch.Register(dispatch.RegisterParams[Req, F, P, D]{Dispatcher: params.Bench.Dispatcher, Command: params.Command, Events: params.Events})
	progress := &Progress{}
	result, err := handle.Run(ctx, dispatch.RunParams[Req]{Request: params.Request, Options: params.Options, Progress: progress})
	return Ran[P, D]{Result: result, Err: err, Progress: progress.Seen()}
}
