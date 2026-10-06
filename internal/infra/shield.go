package infra

import (
	"context"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
)

var inflight = struct {
	sync.Mutex
	cond     *sync.Cond
	open     int
	stopping []func()
}{}

func init() {
	inflight.cond = sync.NewCond(&inflight.Mutex)
}

// Shield is a context no cancellation reaches, for a step that must not stop
// half-way once started: a worktree half removed, a branch left behind its
// worktree. A child started under it runs in its own process group, out of
// reach of the terminal's Ctrl-C, and the process does not exit before release
// is called (AwaitShields).
func Shield(ctx context.Context) (context.Context, func()) {
	inflight.Lock()
	inflight.open++
	inflight.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			inflight.Lock()
			inflight.open--
			inflight.cond.Broadcast()
			inflight.Unlock()
		})
	}
	return context.WithValue(context.WithoutCancel(ctx), domain.ShieldedFrom{}, ctx), release
}

func shielded(ctx context.Context) bool {
	_, on := ctx.Value(domain.ShieldedFrom{}).(context.Context)
	return on
}

// AwaitShields returns once no shielded step is running.
func AwaitShields() {
	inflight.Lock()
	defer inflight.Unlock()
	for inflight.open > 0 {
		inflight.cond.Wait()
	}
}

// KillCancelled kills every child a cancellation is still stopping: one that
// ignores SIGINT would otherwise outlive the process that gave up waiting for it.
func KillCancelled() {
	inflight.Lock()
	kills := inflight.stopping
	inflight.stopping = nil
	inflight.Unlock()
	for _, kill := range kills {
		kill()
	}
}

func stopping(kill func()) {
	inflight.Lock()
	defer inflight.Unlock()
	inflight.stopping = append(inflight.stopping, kill)
}
