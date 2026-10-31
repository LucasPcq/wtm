package daemon_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALegacyDaemonsJobsAreReadAsItSpellsThem(t *testing.T) {
	socket, requests := serve(t, `{"status": "ok", "jobs": [{"name": "web", "status": "detached", "work_dir": "/wt/a"}, {"name": "api", "status": "stopped", "work_dir": "/wt/a"}, {"name": "db", "status": "attached", "work_dir": "/wt/b"}]}`)

	jobs, err := daemon.Daemon{SocketPath: socket}.Jobs(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []run.Job{
		{Name: "web", WorkDir: "/wt/a", Up: true, State: run.Running, Hold: run.HoldDetached},
		{Name: "api", WorkDir: "/wt/a", State: run.Stopped, Hold: run.HoldOwn},
		{Name: "db", WorkDir: "/wt/b", Up: true, Hold: run.HoldClaim},
	}, jobs)
	assert.Equal(t, map[string]any{"action": "list"}, <-requests)
}

func TestALegacyJobIsReadWithHowItStandsAndAStateTheDaemonSaysWins(t *testing.T) {
	socket, _ := serve(t, `{"status": "ok", "jobs": [{"name": "api", "kind": "service", "status": "crashed", "state": "crashed", "work_dir": "/wt/a",
		"url": "http://api.a.acme.localhost", "exit_code": 1, "started_at": "2026-10-16T12:00:00Z", "shared_dir": "/repo"},
		{"name": "migrate", "kind": "task", "status": "stopped", "state": "exited", "work_dir": "/wt/a"}]}`)

	jobs, err := daemon.Daemon{SocketPath: socket}.Jobs(t.Context())

	require.NoError(t, err)
	exit := 1
	assert.Equal(t, []run.Job{
		{Name: "api", WorkDir: "/wt/a", Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn, URL: "http://api.a.acme.localhost", ExitCode: &exit, StartedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), SharedDir: "/repo"},
		{Name: "migrate", WorkDir: "/wt/a", Kind: project.Task, State: run.Exited, Hold: run.HoldOwn},
	}, jobs)
}

func TestNoDaemonAnsweringIsNoJobUnlessItsIndexHoldsAForegroundOne(t *testing.T) {
	cases := map[string]struct {
		index string
		err   error
	}{
		"no_index":    {},
		"empty_index": {index: `{"version": 2, "jobs": []}`},
		"torn_index":  {index: `{"version": 2, "jobs": [`},
		"foreground":  {index: `{"version": 2, "jobs": [{"name": "web", "work_dir": "/wt/a", "config": {"name": "web", "kind": "service", "cmd": "pnpm dev"}}]}`, err: run.ErrUnknown},
		"unreadable":  {index: `{"version": 2, "jobs": [{"name": "web"}]}`, err: run.ErrUnknown},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			jobs, err := daemon.Daemon{SocketPath: unanswered(t, c.index)}.Jobs(t.Context())

			assert.Empty(t, jobs)
			assert.ErrorIs(t, err, c.err)
		})
	}
}

func TestADaemonRefusingTheListingFails(t *testing.T) {
	socket, _ := serve(t, `{"status": "error", "message": "busy"}`)

	_, err := daemon.Daemon{SocketPath: socket}.Jobs(t.Context())

	require.EqualError(t, err, "run daemon: busy")
}

func TestWithNoDaemonTheStacksAndClaimsOfTheIndexAreUp(t *testing.T) {
	socket := unanswered(t, `{"version": 2, "jobs": [
		{"name": "db", "work_dir": "/repo", "config": {"name": "db", "kind": "service", "cmd": "docker compose up -d", "stop": "docker compose down"}, "started_at": "2026-10-31T12:00:00Z"},
		{"name": "db", "work_dir": "/wt/a", "config": {"name": "db", "kind": "service", "cmd": "docker compose up -d", "stop": "docker compose down"}, "joined": true, "shared_dir": "/repo"}]}`)

	jobs, err := daemon.Daemon{SocketPath: socket}.Jobs(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []run.Job{
		{Name: "db", WorkDir: "/repo", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldDetached, StartedAt: started},
		{Name: "db", WorkDir: "/wt/a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/repo"},
	}, jobs)
}

func TestADaemonOfThisProtocolIsReadAsItReports(t *testing.T) {
	claim := run.Job{Name: "db", WorkDir: "/wt/a", Up: true, Kind: project.Service, Hold: run.HoldClaim, Ports: map[string]int{}, SharedDir: "/repo"}
	reaped := run.Job{Name: "worker", WorkDir: "/wt/a", Kind: project.Service, State: run.Stopped, Reaped: true, PID: 77}
	socket, _ := served(t, &jobs{listed: []run.Job{web, claim, reaped}}, "1.0")

	got, err := daemon.Daemon{SocketPath: socket, Version: "0.9"}.Jobs(t.Context())

	require.NoError(t, err)
	claim.Ports = nil
	assert.Equal(t, []run.Job{web, claim, reaped}, got, "a build skew changes nothing of a read")
}

func TestADaemonOfAnotherProtocolIsNotRead(t *testing.T) {
	socket, _ := serve(t, `{"status": "ok", "protocol": 3, "version": "2.0", "jobs": [{"name": "web", "shape": "new"}]}`)

	_, err := daemon.Daemon{SocketPath: socket, Version: "1.0"}.Jobs(t.Context())

	var skew *run.Error
	require.ErrorAs(t, err, &skew)
	assert.Equal(t, run.CodeSkew, skew.Code)
}

func TestAStartHandsOutTheLinesAndReturnsTheJobStarted(t *testing.T) {
	socket, _ := served(t, &jobs{lines: []string{"ready"}}, "1.0")
	var lines []run.Line

	job, err := daemon.Daemon{SocketPath: socket, Version: "1.0"}.Start(t.Context(), run.Spec{Job: project.Job{Name: "web"}}, func(line run.Line) { lines = append(lines, line) })

	require.NoError(t, err)
	assert.Equal(t, web, job)
	assert.Equal(t, []run.Line{{At: started, Text: "ready"}}, lines)
}

func TestAStartTheDaemonRefusesFailsWithItsCodeAndExitCode(t *testing.T) {
	socket, _ := served(t, &jobs{err: &run.Error{Code: run.CodeTaskFailed, Message: "task migrate exited with code 3", ExitCode: new(3)}}, "1.0")

	_, err := daemon.Daemon{SocketPath: socket, Version: "1.0"}.Start(t.Context(), run.Spec{Job: project.Job{Name: "migrate"}}, nil)

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeTaskFailed, failure.Code)
	assert.Equal(t, 3, *failure.ExitCode)
	assert.Equal(t, "run daemon: task migrate exited with code 3", failure.Message)
}

func TestEnsureStartsADaemonWhenNoneAnswers(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	launched := 0
	d := daemon.Daemon{SocketPath: socket, Version: "1.0", StartDaemon: func() error {
		launched++
		servedAt(t, socket, &jobs{}, "1.0")
		return nil
	}}

	require.NoError(t, d.Ensure(t.Context()))

	assert.Equal(t, 1, launched)
}

func TestEnsureLeavesADaemonOfThisBuildAlone(t *testing.T) {
	socket, shutdowns := served(t, &jobs{listed: []run.Job{web}}, "1.0")

	require.NoError(t, daemon.Daemon{SocketPath: socket, Version: "1.0"}.Ensure(t.Context()))

	assert.Zero(t, shutdowns.Load())
}

func TestEnsureReplacesASkewedDaemonHoldingNoJob(t *testing.T) {
	socket, shutdowns := served(t, &jobs{listed: []run.Job{{Name: "web", WorkDir: "/wt/a", State: run.Crashed}}}, "0.9")
	launched := 0
	d := daemon.Daemon{SocketPath: socket, Version: "1.0", StartDaemon: func() error {
		launched++
		servedAt(t, socket, &jobs{}, "1.0")
		return nil
	}}

	require.NoError(t, d.Ensure(t.Context()))

	assert.Equal(t, int32(1), shutdowns.Load())
	assert.Equal(t, 1, launched)
}

func TestEnsureRefusesASkewedDaemonHoldingJobs(t *testing.T) {
	cases := map[string]struct {
		answer string
		code   string
		says   string
	}{
		"build":  {answer: `{"status":"ok","protocol":2,"version":"0.9","jobs":[{"name":"web","work_dir":"/wt/a","state":"running","hold":"own"}]}`, code: run.CodeSkew, says: "the run daemon is wtm 0.9, this is wtm 1.0, and it runs 1 job(s)"},
		"legacy": {answer: `{"status":"ok","version":"0.30.1","daemon_pid":12,"jobs":[{"name":"web","status":"running"},{"name":"db","status":"detached"},{"name":"api","status":"crashed"}]}`, code: run.CodeLegacy, says: "the run daemon is the legacy wtm 0.30.1, this is wtm 1.0, and it runs 2 job(s)"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			socket, requests := serve(t, c.answer)

			err := daemon.Daemon{SocketPath: socket, Version: "1.0", StartDaemon: func() error { t.Fatal("started a daemon"); return nil }}.Ensure(t.Context())

			var refusal *run.Error
			require.ErrorAs(t, err, &refusal)
			assert.Equal(t, c.code, refusal.Code)
			assert.Equal(t, c.says, refusal.Message)
			assert.Equal(t, map[string]any{"action": "list"}, <-requests)
			assert.Empty(t, requests, "the daemon holding jobs is neither stopped nor signalled")
		})
	}
}

func TestEnsureWaitsForAProcessThatIsNoRunDaemonThenRefusesIt(t *testing.T) {
	defer daemon.ShortenStartWait(100 * time.Millisecond)()
	socket, requests := serve(t, `{"status":"error","message":"unknown action: list"}`)
	lock := holdLock(t, filepath.Join(filepath.Dir(socket), "wtm.lock"))
	defer lock.Close()

	err := daemon.Daemon{SocketPath: socket, Version: "1.0", StartDaemon: func() error { t.Fatal("started a daemon"); return nil }}.Ensure(t.Context())

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeBusy, refusal.Code)
	<-requests
	assert.Empty(t, requests, "it is never asked to stop, nor signalled")
}

func TestASocketPathPastTheSystemsLimitIsRefusedBeforeAnythingStarts(t *testing.T) {
	socket := filepath.Join(socketDir(t), strings.Repeat("x", 120), "wtm.sock")

	err := daemon.Daemon{SocketPath: socket, StartDaemon: func() error { t.Fatal("started a daemon"); return nil }}.Ensure(t.Context())

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeSocketTooLong, refusal.Code)
}

func TestADaemonThatDoesNotStartSaysWhyFromItsLog(t *testing.T) {
	defer daemon.ShortenStartWait(100 * time.Millisecond)()
	socket := filepath.Join(socketDir(t), "wtm.sock")
	d := daemon.Daemon{SocketPath: socket, StartDaemon: func() error {
		return os.WriteFile(daemon.LogPath(socket), []byte("starting\nError: listen unix: bind: invalid argument\n"), 0o600)
	}}

	err := d.Ensure(t.Context())

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeStartFailed, failure.Code)
	assert.Equal(t, "the run daemon did not start within 100ms: Error: listen unix: bind: invalid argument", failure.Message)
}

func TestAStopWithNoDaemonStartsOneOnlyForAJobTheIndexHolds(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	require.NoError(t, os.WriteFile(daemon.IndexPath(socket), []byte(`{"version": 2, "jobs": [{"name": "db", "work_dir": "/wt/a", "config": {"name": "db", "kind": "service", "cmd": "up", "stop": "down"}}]}`), 0o600))
	launched := 0
	d := daemon.Daemon{SocketPath: socket, StartDaemon: func() error {
		launched++
		servedAt(t, socket, &jobs{outcome: run.OutcomeStopped}, "1.0")
		return nil
	}}

	elsewhere, err := d.Stop(t.Context(), run.Stop{Name: "db", WorkDir: "/wt/b"})
	require.NoError(t, err)
	assert.Equal(t, run.OutcomeNotRunning, elsewhere)
	assert.Zero(t, launched)

	stopped, err := d.Stop(t.Context(), run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, stopped)
	assert.Equal(t, 1, launched)
}

func TestALegacyDaemonsStopIsReadAsStopped(t *testing.T) {
	socket, requests := serve(t, `{"status":"ok","version":"0.30.1","message":"job web stopped"}`)

	outcome, err := daemon.Daemon{SocketPath: socket}.Stop(t.Context(), run.Stop{Name: "web", WorkDir: "/wt/a", CorrelationID: "c1"})

	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, outcome)
	assert.Equal(t, map[string]any{"action": "stop", "name": "web", "work_dir": "/wt/a", "correlation_id": "c1"}, <-requests)
}

func TestStopAllReturnsEachJobWithWhatTheStopDid(t *testing.T) {
	socket, _ := served(t, &jobs{}, "1.0")

	stopped, err := daemon.Daemon{SocketPath: socket}.StopAll(t.Context(), run.StopAll{WorkDir: "/wt/a"})

	require.NoError(t, err)
	assert.Equal(t, []run.StoppedJob{{Job: web, Outcome: run.OutcomeStopped}}, stopped)
}

func TestShutdownAsksTheDaemonAndWaitsForItsLock(t *testing.T) {
	socket, shutdowns := served(t, &jobs{}, "1.0")

	require.NoError(t, daemon.Daemon{SocketPath: socket}.Shutdown(t.Context()))
	require.NoError(t, daemon.Daemon{SocketPath: socket}.Shutdown(t.Context()), "none answering is no error")

	assert.Equal(t, int32(1), shutdowns.Load())
}

func TestEnsureTakesEveryJobADaemonOfAnotherProtocolListsAsHeld(t *testing.T) {
	socket, requests := serve(t, `{"status":"ok","protocol":3,"version":"2.0","jobs":[{"name":"web","shape":"new"}]}`)

	err := daemon.Daemon{SocketPath: socket, Version: "1.0", StartDaemon: func() error { t.Fatal("started a daemon"); return nil }}.Ensure(t.Context())

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeSkew, refusal.Code)
	<-requests
	assert.Empty(t, requests, "it is not asked to stop")
}

func TestLinesHandsOutEachLineAndReturnsTheJob(t *testing.T) {
	socket, _ := served(t, &jobs{lines: []string{"ready"}}, "1.0")
	var lines []run.Line

	job, err := daemon.Daemon{SocketPath: socket}.Lines(t.Context(), run.Tail{Name: "web", WorkDir: "/wt/a", Lines: 10}, func(line run.Line) { lines = append(lines, line) })

	require.NoError(t, err)
	assert.Equal(t, web, job)
	assert.Equal(t, []run.Line{{At: started, Text: "ready"}}, lines)
}

func TestAFollowEndsWhenItsReaderLeaves(t *testing.T) {
	socket, _ := servedConcurrently(t, &jobs{lines: []string{"ready"}})
	ctx, cancel := context.WithCancel(t.Context())
	got := make(chan string, 1)

	go func() {
		_, err := daemon.Daemon{SocketPath: socket}.Lines(ctx, run.Tail{Name: "web", WorkDir: "/wt/a", Follow: true}, func(line run.Line) { got <- line.Text })
		got <- err.Error()
	}()
	assert.Equal(t, "ready", <-got)
	cancel()

	assert.Contains(t, <-got, "context canceled")
}

func TestAnAttachedTerminalReadsTheJobAndTypesIntoIt(t *testing.T) {
	daemonJobs := &jobs{}
	socket, _ := servedConcurrently(t, daemonJobs)

	conn, err := daemon.Daemon{SocketPath: socket, Version: "1.0"}.Attach(t.Context(), run.Window{Name: "web", WorkDir: "/wt/a", Cols: 80, Rows: 24})
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("q"))
	require.NoError(t, err)
	read, err := io.ReadAll(conn)

	require.NoError(t, err)
	assert.Equal(t, "history|live", string(read))
	assert.Eventually(t, func() bool { return daemonJobs.typed.String() == "q" }, time.Second, time.Millisecond)
	assert.Equal(t, []any{run.Window{Name: "web", WorkDir: "/wt/a", Cols: 80, Rows: 24}}, daemonJobs.seen())
}

func TestAnAttachTheDaemonRefusesFailsWithItsCode(t *testing.T) {
	socket, _ := served(t, &jobs{err: &run.Error{Code: run.CodeJobNotFound, Message: "no job web running in /wt/a"}}, "1.0")

	_, err := daemon.Daemon{SocketPath: socket, Version: "1.0"}.Attach(t.Context(), run.Window{Name: "web", WorkDir: "/wt/a"})

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeJobNotFound, refusal.Code)
}

func TestAResizeTheDaemonRefusesFailsWithItsCode(t *testing.T) {
	socket, _ := served(t, &jobs{err: &run.Error{Code: run.CodeNoTerminal, Message: "job migrate runs under no terminal"}}, "1.0")

	err := daemon.Daemon{SocketPath: socket, Version: "1.0"}.Resize(t.Context(), run.Window{Name: "migrate", WorkDir: "/wt/a", Cols: 80, Rows: 24})

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeNoTerminal, refusal.Code)
}

func TestAnAttachedTerminalKeepsReceivingPastAWritesDeadline(t *testing.T) {
	socket, _ := servedConcurrently(t, &jobs{lateAfter: 1500 * time.Millisecond})

	conn, err := daemon.Daemon{SocketPath: socket, Version: "1.0"}.Attach(t.Context(), run.Window{Name: "web", WorkDir: "/wt/a"})
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("q"))
	require.NoError(t, err)
	read, err := io.ReadAll(conn)

	require.NoError(t, err)
	assert.Equal(t, "history|live|late", string(read), "what the job prints a while after the attach still arrives")
}

func TestAnAttachOrAResizeOfADaemonOfAnotherBuildIsRefused(t *testing.T) {
	socket, _ := servedConcurrently(t, &jobs{})
	skewed := daemon.Daemon{SocketPath: socket, Version: "2.0"}

	_, attach := skewed.Attach(t.Context(), run.Window{Name: "web", WorkDir: "/wt/a"})
	resize := skewed.Resize(t.Context(), run.Window{Name: "web", WorkDir: "/wt/a", Cols: 80, Rows: 24})

	for _, err := range []error{attach, resize} {
		var refusal *run.Error
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, run.CodeSkew, refusal.Code)
	}
}
