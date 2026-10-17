package shell

import (
	"context"
	"maps"
	"slices"
	"sync"
)

var inflight = struct {
	sync.Mutex
	cond     *sync.Cond
	open     int
	stopping map[*escalation]func()
}{stopping: map[*escalation]func(){}}

func init() {
	inflight.cond = sync.NewCond(&inflight.Mutex)
}

func Shield(ctx context.Context) (context.Context, func()) {
	inflight.Lock()
	inflight.open++
	inflight.Unlock()
	release := sync.OnceFunc(func() {
		inflight.Lock()
		inflight.open--
		inflight.cond.Broadcast()
		inflight.Unlock()
	})
	return context.WithoutCancel(ctx), release
}

func AwaitShields() {
	inflight.Lock()
	defer inflight.Unlock()
	for inflight.open > 0 {
		inflight.cond.Wait()
	}
}

func KillCancelled() {
	inflight.Lock()
	kills := slices.Collect(maps.Values(inflight.stopping))
	inflight.Unlock()
	for _, kill := range kills {
		kill()
	}
}

type stoppingParams struct {
	Process *escalation
	Kill    func()
}

func stopping(params stoppingParams) {
	inflight.Lock()
	defer inflight.Unlock()
	inflight.stopping[params.Process] = params.Kill
}

func stopped(process *escalation) {
	inflight.Lock()
	defer inflight.Unlock()
	delete(inflight.stopping, process)
}
