package daemon_test

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsAreThoseTheDaemonLists(t *testing.T) {
	socket, requests := serve(t, `{"status": "ok", "jobs": [{"name": "web", "status": "detached", "work_dir": "/wt/a"}, {"name": "api", "status": "stopped", "work_dir": "/wt/a"}, {"name": "db", "status": "attached", "work_dir": "/wt/b"}]}`)

	jobs, err := daemon.Daemon{SocketPath: socket}.Jobs(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []run.Job{
		{Name: "web", WorkDir: "/wt/a", Up: true, State: run.Running},
		{Name: "api", WorkDir: "/wt/a", State: run.Stopped},
		{Name: "db", WorkDir: "/wt/b", Up: true, Claim: true},
	}, jobs)
	assert.Equal(t, map[string]any{"action": "list"}, <-requests)
}

func TestAJobIsReadWithHowItStandsAndAStateTheDaemonSaysWins(t *testing.T) {
	socket, _ := serve(t, `{"status": "ok", "jobs": [{"name": "api", "kind": "service", "status": "crashed", "state": "crashed", "work_dir": "/wt/a",
		"url": "http://api.a.acme.localhost", "exit_code": 1, "started_at": "2026-10-16T12:00:00Z", "shared_dir": "/repo"},
		{"name": "migrate", "kind": "task", "status": "stopped", "state": "exited", "work_dir": "/wt/a"}]}`)

	jobs, err := daemon.Daemon{SocketPath: socket}.Jobs(t.Context())

	require.NoError(t, err)
	exit := 1
	assert.Equal(t, []run.Job{
		{Name: "api", WorkDir: "/wt/a", Kind: project.Service, State: run.Crashed, URL: "http://api.a.acme.localhost", ExitCode: &exit, StartedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), SharedDir: "/repo"},
		{Name: "migrate", WorkDir: "/wt/a", Kind: project.Task, State: run.Exited},
	}, jobs)
}

func TestNoDaemonAnsweringIsNoJobUnlessItsIndexHoldsSome(t *testing.T) {
	cases := map[string]struct {
		index string
		err   error
	}{
		"no_index":    {},
		"empty_index": {index: `{"version": 2, "jobs": []}`},
		"torn_index":  {index: `{"version": 2, "jobs": [`},
		"held_jobs":   {index: `{"version": 2, "jobs": [{"name": "web"}]}`, err: run.ErrUnknown},
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
