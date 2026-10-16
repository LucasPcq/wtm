package shell

import (
	"context"
	"sync"
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
