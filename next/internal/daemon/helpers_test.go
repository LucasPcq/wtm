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
	logs    *logs
	gone    map[string]bool
}

func newWorld(t *testing.T, scripts map[string]script) *world {
	t.Helper()
	w := &world{spawner: newSpawner(scripts), orphans: &orphans{}, index: &index{}, bus: &bus{}, logs: newLogs(), gone: map[string]bool{}}
	w.Daemon = New(Params{
		Spawn:   w.spawner.spawn,
		Orphans: w.orphans,
		Index:   w.index,
		OpenLog: w.logs.open,
		ReadLog: w.logs.read,
		Exists:  func(path string) bool { return !w.gone[path] },
		Publish: w.bus.publish,
		Now:     func() time.Time { return now },
		Grace:   time.Millisecond,

		NamespaceBudget: 50 * time.Millisecond,
		NamespaceRetry:  time.Millisecond,
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

// relaunching starts the stack db, then starts it again with a launcher that runs until it is stopped, and returns the job of the first launch.
func (w *world) relaunching(t *testing.T) *job {
	t.Helper()
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	w.mu.Lock()
	first := w.jobs[key{workDir: "/wt/a", name: "db"}]
	w.mu.Unlock()
	w.spawner.mu.Lock()
	delete(w.spawner.scripts, "up db")
	w.spawner.mu.Unlock()
	go func() { _, _ = w.Start(stack("db"), nil) }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 2 }, time.Second, time.Millisecond)
	return first
}

// given waits until the worktree /wt/a holds j as its job db again: the launcher stopped gave it back.
func (w *world) given(t *testing.T, j *job) {
	t.Helper()
	require.Eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.jobs[key{workDir: "/wt/a", name: "db"}] == j
	}, time.Second, time.Millisecond)
}
