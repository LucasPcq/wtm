package daemon

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/require"
)

// world is a daemon over fakes, and the fakes.
type world struct {
	*Daemon
	spawner *spawner
	orphans *orphans
	index   *index
	bus     *bus
	gone    map[string]bool
}

func newWorld(t *testing.T, scripts map[string]script) *world {
	t.Helper()
	w := &world{spawner: newSpawner(scripts), orphans: &orphans{}, index: &index{}, bus: &bus{}, gone: map[string]bool{}}
	w.Daemon = New(Params{
		Spawn:   w.spawner.spawn,
		Orphans: w.orphans,
		Index:   w.index,
		Exists:  func(path string) bool { return !w.gone[path] },
		Publish: w.bus.publish,
		Now:     func() time.Time { return now },
		Grace:   time.Millisecond,
	})
	return w
}

// job returns the job name of the worktree /wt/a as the daemon lists it.
func (w *world) job(t *testing.T, name string) run.Job {
	t.Helper()
	for _, job := range w.List("/wt/a") {
		if job.Name == name {
			return job
		}
	}
	require.Failf(t, "no such job", "%s is not listed", name)
	return run.Job{}
}

// settled waits until the job name of /wt/a is in state.
func (w *world) settled(t *testing.T, name string, state run.State) run.Job {
	t.Helper()
	require.Eventually(t, func() bool { return w.job(t, name).State == state }, 2*time.Second, time.Millisecond)
	return w.job(t, name)
}
