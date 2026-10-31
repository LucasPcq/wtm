package daemon

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAServiceUpIsServedUnderItsNameAndListedByItUntilItStops(t *testing.T) {
	w := newWorld(t, nil)
	w.routes.listen(11080)

	started, err := w.Start(routed(service("web")), nil)
	require.NoError(t, err)
	served := w.routes.table()
	_, err = w.Stop(run.Stop{Name: "web", WorkDir: "/wt/a"})
	require.NoError(t, err)

	assert.Equal(t, map[string]int{"web.feat-a.acme.localhost": 3100}, served)
	assert.Equal(t, "http://web.feat-a.acme.localhost:11080", started.URL)
	event, ok := w.bus.events()[0].(domain.JobStarted)
	require.True(t, ok)
	assert.Equal(t, "http://web.feat-a.acme.localhost:11080", event.Job.URL)
	assert.Empty(t, w.routes.table(), "withdrawn once it stopped")
}

func TestAServiceThatCrashesIsNoLongerServed(t *testing.T) {
	w := newWorld(t, nil)
	_, err := w.Start(routed(service("web")), nil)
	require.NoError(t, err)

	w.spawner.process("serve web").end(1)

	require.Eventually(t, func() bool { return len(w.routes.table()) == 0 }, time.Second, time.Millisecond)
}

func TestAStackIsServedOnceLaunchedAndWithdrawnOnceItsStopCommandRan(t *testing.T) {
	up := make(chan struct{})
	w := newWorld(t, map[string]script{"up db": holds(up, 0), "down db": exits(0)})
	launched := make(chan error, 1)
	go func() {
		_, err := w.Start(routed(stack("db")), nil)
		launched <- err
	}()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 1 }, time.Second, time.Millisecond)
	launching := w.routes.table()

	close(up)
	require.NoError(t, <-launched)
	served := w.routes.table()
	_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)

	assert.Empty(t, launching, "nothing answers while its launcher runs")
	assert.Equal(t, map[string]int{"db.feat-a.acme.localhost": 3100}, served)
	assert.Empty(t, w.routes.table())
}

func TestAStackWhoseStopCommandFailsStaysServed(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(1)})
	_, err := w.Start(routed(stack("db")), nil)
	require.NoError(t, err)

	_, err = w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	require.Error(t, err)
	assert.Equal(t, map[string]int{"db.feat-a.acme.localhost": 3100}, w.routes.table())
}

func TestAStackTakenOverIsServedAgain(t *testing.T) {
	w := newWorld(t, nil)
	w.index.records = []run.Record{{Spec: routed(stack("db")), StartedAt: now}}

	w.Adopt()

	assert.Equal(t, map[string]int{"db.feat-a.acme.localhost": 3100}, w.routes.table())
}

func TestASharedJobIsServedOnceForEveryWorktreeHoldingIt(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	for _, at := range []string{"/wt/a", "/wt/b"} {
		spec := routed(shared("db", at, "down db"))
		spec.Routes[0].Host = "db.acme.localhost"
		spec.Shared.Env = spec.Env
		_, err := w.Start(spec, nil)
		require.NoError(t, err)
	}

	served := w.routes.table()
	_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)
	released := w.routes.table()
	_, err = w.Stop(run.Stop{Name: "db", WorkDir: "/wt/b"})
	require.NoError(t, err)

	assert.Equal(t, map[string]int{"db.acme.localhost": 3100}, served)
	assert.Equal(t, served, released, "a claim let go of withdraws nothing")
	assert.Empty(t, w.routes.table())
}

func TestTheDaemonStaysUpWhileItsProxyServesAStack(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "up cache": exits(0)})
	w.routes.listen(11080)
	_, err := w.Start(stack("cache"), nil)
	require.NoError(t, err)
	idle := !w.Busy()

	_, err = w.Start(routed(stack("db")), nil)
	require.NoError(t, err)

	assert.True(t, idle, "a stack nothing serves leaves it free to stop")
	assert.True(t, w.Busy())
	w.routes.listen(0)
	assert.False(t, w.Busy(), "a proxy that serves on no port serves no name")
}

func TestAStopCommandThatHangsIsStoppedAndLeavesTheStackUp(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	var limits []time.Duration
	w.params.After = func(limit time.Duration) <-chan time.Time {
		limits = append(limits, limit)
		fell := make(chan time.Time, 1)
		fell <- now
		return fell
	}
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)

	_, err = w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeStopFailed, failure.Code)
	assert.Equal(t, "the stop command of db was still running after 5m0s, and was stopped", failure.Message)
	assert.Equal(t, []time.Duration{5 * time.Minute}, limits)
	assert.NotEmpty(t, w.spawner.process("down db").stops)
	assert.True(t, w.job(t, "db").Up, "left up and indexed")
}

func TestAStopRacingAStackLaunchedAgainLeavesTheNewStackServed(t *testing.T) {
	down := make(chan struct{})
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": holds(down, 0)})
	w.routes.listen(11080)
	_, err := w.Start(routed(stack("db")), nil)
	require.NoError(t, err)
	stopped := make(chan error, 1)
	go func() {
		_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
		stopped <- err
	}()
	require.Eventually(t, func() bool { return w.spawner.process("down db") != nil }, time.Second, time.Millisecond)

	_, err = w.Start(routed(stack("db")), nil)
	require.NoError(t, err)
	close(down)

	require.NoError(t, <-stopped)
	assert.Equal(t, map[string]int{"db.feat-a.acme.localhost": 3100}, w.routes.table(), "the old stack's stop withdraws nothing of the new one")
	assert.True(t, w.Busy())
}

func TestAStackALauncherStoppedLeftUpIsServedWhenItsStopCommandFails(t *testing.T) {
	w := newWorld(t, map[string]script{"down db": exits(1)})
	go func() { _, _ = w.Start(routed(stack("db")), nil) }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 1 }, time.Second, time.Millisecond)

	_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	require.Error(t, err)
	assert.True(t, w.job(t, "db").Up)
	assert.Equal(t, map[string]int{"db.feat-a.acme.localhost": 3100}, w.routes.table())
}

func TestAStopCommandAShutdownCutsShortSaysSo(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	stopped := make(chan error, 1)
	go func() {
		_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
		stopped <- err
	}()
	require.Eventually(t, func() bool { return w.spawner.process("down db") != nil }, time.Second, time.Millisecond)

	w.Shutdown()

	var failure *run.Error
	require.ErrorAs(t, <-stopped, &failure)
	assert.Equal(t, "the stop command of db was stopped as the run daemon stopped", failure.Message)
}

func TestTheProxysPublicPortIs80OnceTheRedirectionReachesIt(t *testing.T) {
	w := newWorld(t, nil)
	w.routes.listen(11080)
	w.params.Probe = func(port int) int {
		if port == run.PrivilegedPort {
			return 11080
		}
		return 0
	}

	web, err := w.Start(routed(service("web")), nil)

	require.NoError(t, err)
	assert.Equal(t, run.Proxy{Port: 11080, PublicPort: run.PrivilegedPort}, w.Proxy())
	assert.Equal(t, "http://web.feat-a.acme.localhost", web.URL)
}

func TestAStackGivenBackAfterAFailedLaunchIsServed(t *testing.T) {
	down, failing := make(chan struct{}), make(chan struct{})
	var launches atomic.Int32
	up := func(p *fakeProcess, _ func(string)) {
		if launches.Add(1) == 1 {
			return
		}
		<-failing
		p.end(1)
	}
	w := newWorld(t, map[string]script{"up db": up, "down db": holds(down, 1)})
	w.routes.listen(11080)
	go func() { _, _ = w.Start(routed(stack("db")), nil) }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 1 }, time.Second, time.Millisecond)
	stopped := make(chan error, 1)
	go func() {
		_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
		stopped <- err
	}()
	require.Eventually(t, func() bool { return w.spawner.process("down db") != nil }, time.Second, time.Millisecond)
	relaunched := make(chan error, 1)
	go func() {
		_, err := w.Start(routed(stack("db")), nil)
		relaunched <- err
	}()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 3 }, time.Second, time.Millisecond)

	close(down)
	require.Error(t, <-stopped, "its stop command fails while the launch again holds the worktree's job")
	close(failing)
	require.Error(t, <-relaunched)

	assert.True(t, w.job(t, "db").Up, "the stack the stopped launcher left, given back")
	assert.Equal(t, map[string]int{"db.feat-a.acme.localhost": 3100}, w.routes.table())
}
