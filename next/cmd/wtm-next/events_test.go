package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/registry"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Legacy followed an inherited $GIT_DIR in its git calls: the snapshot described the repository it pointed to.
func TestTheEventStreamOfASurfaceOpensOnTheCallsRepositoryWhateverGitDirSays(t *testing.T) {
	main, other := configured(t), configured(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	stream := api.BindStream[events.Request, events.Questions](events.Name, watchEvents(eventStream(streamParams{Socket: servingBus(t), Registry: filepath.Join(t.TempDir(), "repos.json")})))

	lines, response := watchedLines(t, stream, api.Call{Dir: main}, 2)

	assert.Equal(t, http.StatusOK, response.Status, response.Envelope.Err)
	require.Len(t, lines, 2)
	var snapshot struct {
		Type      string      `json:"type"`
		Repo      domain.Repo `json:"repo"`
		Worktrees []struct {
			Branch string `json:"branch"`
			Path   string `json:"path"`
		} `json:"worktrees"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &snapshot))
	assert.Equal(t, "snapshot", snapshot.Type)
	assert.Equal(t, main, snapshot.Repo.Root)
	require.Len(t, snapshot.Worktrees, 1)
	assert.Equal(t, main, snapshot.Worktrees[0].Path)
	assert.JSONEq(t, `{"v":2,"type":"ready","ts":"x"}`, withTS(t, lines[1]))
}

func TestAChangeInAnotherProcessReachesTheStream(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")
	socket := servingBus(t)
	stream := api.BindStream[events.Request, events.Questions](events.Name, watchEvents(eventStream(streamParams{Socket: socket, Registry: filepath.Join(t.TempDir(), "repos.json")})))
	params, err := locate(t.Context(), main)
	require.NoError(t, err)
	publisher := params.publisherParams(domain.Operation{ID: "op-1"})
	publisher.SocketPath = socket

	var lines []string
	done := make(chan api.Response, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready := make(chan struct{})
	go func() {
		done <- stream.Watch(ctx, api.Call{Dir: main}, func(line json.RawMessage) error {
			lines = append(lines, string(line))
			switch len(lines) {
			case 2:
				close(ready)
			case 3:
				cancel()
			}
			return nil
		})
	}()
	<-ready
	bus.NewPublisher(publisher).Publish(domain.OperationStarted{Command: "create", Surface: "cli"})

	<-done
	require.Len(t, lines, 3)
	assert.Contains(t, lines[2], `"type":"operation.started"`)
	assert.Contains(t, lines[2], `"op_id":"op-1"`)
}

func TestOpeningARepositoryRegistersItAmongThoseWtmKnows(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")

	_, err := openList(t.Context(), main)
	require.NoError(t, err)

	data, err := os.ReadFile(registry.Path())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"common_dir": "`+filepath.Join(gittest.Resolved(t, main), ".git")+`"`)
}

func TestARepositoryWhoseStateIsMovedElsewhereIsNotRegistered(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, t.TempDir())

	_, err := openList(t.Context(), main)
	require.NoError(t, err)

	assert.NoFileExists(t, registry.Path())
}

func TestNoDaemonIsStartedFromATest(t *testing.T) {
	assert.ErrorIs(t, startDaemon(), errDaemonInTest)
}

func TestTheDaemonIsAHiddenCommand(t *testing.T) {
	streams, _, _, _ := output.Test()
	root := cli.NewRoot(version, streams, commands(streams, cli.Env{}))

	daemon, _, err := root.Find([]string{daemonCommand})

	require.NoError(t, err)
	assert.Equal(t, daemonCommand, daemon.Name())
	assert.True(t, daemon.Hidden)
}
