package shell

import (
	"context"
	"maps"
	"slices"
	"sync"
)

// inflight is the process-wide state of cancellation: how many shielded
// sections are open, and how to kill each cancelled child still stopping.
var inflight = struct {
	sync.Mutex
	cond     *sync.Cond
	open     int
	stopping map[*escalation]func()
}{stopping: map[*escalation]func(){}}

func init() {
	inflight.cond = sync.NewCond(&inflight.Mutex)
}

// Shield opens a section that a first cancellation must let finish: it returns
// ctx without its cancellation and the function that closes the section, safe
// to call more than once.
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

// AwaitShields blocks until every section opened by Shield is released.
func AwaitShields() {
	inflight.Lock()
	defer inflight.Unlock()
	for inflight.open > 0 {
		inflight.cond.Wait()
	}
}

// KillCancelled sends SIGKILL now to the group of every cancelled child still
// stopping, as a second Ctrl+C does.
func KillCancelled() {
	inflight.Lock()
	kills := slices.Collect(maps.Values(inflight.stopping))
	inflight.Unlock()

	for _, kill := range kills {
		kill()
	}
}

func stopping(process *escalation, kill func()) {
	inflight.Lock()
	defer inflight.Unlock()
	inflight.stopping[process] = kill
}

func stopped(process *escalation) {
	inflight.Lock()
	defer inflight.Unlock()
	delete(inflight.stopping, process)
}
