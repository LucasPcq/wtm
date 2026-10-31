package main

import (
	"os"
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
