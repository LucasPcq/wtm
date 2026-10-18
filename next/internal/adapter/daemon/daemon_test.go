package daemon_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsAreThoseTheDaemonLists(t *testing.T) {
	socket, requests := serve(t, `{"status": "ok", "jobs": [{"name": "web", "status": "detached", "work_dir": "/wt/a"}, {"name": "api", "status": "stopped", "work_dir": "/wt/a"}, {"name": "db", "status": "attached", "work_dir": "/wt/b"}]}`)

	jobs, err := daemon.Daemon{SocketPath: socket}.Jobs(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []run.Job{{Name: "web", WorkDir: "/wt/a", Up: true}, {Name: "api", WorkDir: "/wt/a"}, {Name: "db", WorkDir: "/wt/b", Up: true}}, jobs)
	assert.Equal(t, map[string]any{"action": "list"}, <-requests)
}

func TestNoDaemonAnsweringIsNoJobUnlessItsIndexHoldsSome(t *testing.T) {
	nowhere := filepath.Join(t.TempDir(), "wtm.sock")
	cases := map[string]struct {
		index string
		err   error
	}{
		"no_index":    {index: filepath.Join(t.TempDir(), "jobs.json")},
		"empty_index": {index: index(t, `{"version": 2, "jobs": []}`)},
		"torn_index":  {index: index(t, `{"version": 2, "jobs": [`)},
		"held_jobs":   {index: index(t, `{"version": 2, "jobs": [{"name": "web"}]}`), err: run.ErrUnknown},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			jobs, err := daemon.Daemon{SocketPath: nowhere, IndexPath: c.index}.Jobs(t.Context())

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
