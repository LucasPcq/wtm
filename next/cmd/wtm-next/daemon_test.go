package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheDaemonRunsStopsAndAnnouncesTheJobsItsClientsAsk(t *testing.T) {
	socket := servingDaemon(t)
	client := daemon.Daemon{SocketPath: socket, Version: version}
	lines, err := bus.Bus{SocketPath: socket}.Subscribe(t.Context(), nil)
	require.NoError(t, err)
	dir := t.TempDir()
	origin := &run.Origin{Root: "/repo", CommonDir: "/repo/.git", Branch: "feat/a"}

	var printed []string
	migrate, err := client.Start(t.Context(), run.Spec{Job: project.Job{Name: "migrate", Kind: project.Task, Cmd: "echo migrated"}, WorkDir: dir, Origin: origin}, func(line run.Line) { printed = append(printed, line.Text) })
	require.NoError(t, err)
	web, err := client.Start(t.Context(), run.Spec{Job: project.Job{Name: "web", Kind: project.Service, Cmd: "sleep 30"}, WorkDir: dir, Origin: origin}, nil)
	require.NoError(t, err)
	listed, err := client.Jobs(t.Context())
	require.NoError(t, err)
	outcome, err := client.Stop(t.Context(), run.Stop{Name: "web", WorkDir: dir, CorrelationID: "stop-1"})
	require.NoError(t, err)

	assert.Equal(t, run.Exited, migrate.State)
	assert.Equal(t, []string{"migrated"}, printed)
	assert.True(t, web.Up)
	assert.Positive(t, web.PID)
	assert.Equal(t, []string{"web"}, run.UpIn(listed, dir))
	assert.Equal(t, run.OutcomeStopped, outcome)
	var announced []string
	for range 4 {
		select {
		case payload := <-lines:
			line, err := domain.UnmarshalLine(payload)
			require.NoError(t, err)
			announced = append(announced, domain.EventType(line.Event)+" "+line.CorrelationID)
		case <-time.After(5 * time.Second):
			require.Fail(t, "an event is missing", "announced so far: %v", announced)
		}
	}
	assert.Equal(t, []string{"job.started ", "job.exited ", "job.started ", "job.stopped stop-1"}, announced)
	index, err := os.ReadFile(daemon.IndexPath(socket))
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":2,"jobs":[]}`, string(index))
}

func TestTheDaemonKeepsAJobsLogAndHandsOutItsLinesAndItsTerminal(t *testing.T) {
	socket := servingDaemon(t)
	client := daemon.Daemon{SocketPath: socket, Version: version}
	dir, logs := t.TempDir(), t.TempDir()
	spec := run.Spec{Job: project.Job{Name: "echo", Kind: project.Service, Cmd: "stty -echo; echo ready; exec cat"}, WorkDir: dir, LogDir: logs}
	_, err := client.Start(t.Context(), spec, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		jobs, err := client.Jobs(t.Context())
		return err == nil && len(jobs) == 1 && jobs[0].LastLine != nil && jobs[0].LastLine.Text == "ready"
	}, 5*time.Second, 20*time.Millisecond)

	followed := make(chan string, 8)
	go func() {
		_, _ = client.Lines(t.Context(), run.Tail{Name: "echo", WorkDir: dir, Follow: true}, func(line run.Line) { followed <- line.Text })
		close(followed)
	}()
	terminal, err := client.Attach(t.Context(), run.Window{Name: "echo", WorkDir: dir, Cols: 80, Rows: 24})
	require.NoError(t, err)
	defer terminal.Close()
	_, err = terminal.Write([]byte("typed\n"))
	require.NoError(t, err)

	assert.Equal(t, "ready", <-followed)
	assert.Equal(t, "typed", <-followed)
	seen := make([]byte, 64)
	n, err := io.ReadAtLeast(terminal, seen, len("ready"))
	require.NoError(t, err)
	assert.Contains(t, string(seen[:n]), "ready")
	_, err = client.Stop(t.Context(), run.Stop{Name: "echo", WorkDir: dir})
	require.NoError(t, err)
	for range followed {
	}
	log, err := os.ReadFile(filepath.Join(logs, "echo.log"))
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^\S+  ready\n\S+  typed\n$`, string(log))
}
