package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/registry"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	eventscli "github.com/LucasPcq/wtm/next/internal/command/events/cli"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Legacy followed an inherited $GIT_DIR in its git calls: the snapshot described the repository it pointed to.
func TestTheEventStreamOfASurfaceOpensOnTheCallsRepositoryWhateverGitDirSays(t *testing.T) {
	main, other := configured(t), configured(t)
	ownConfigDir(t)
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
	ownConfigDir(t)
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
	ownConfigDir(t)
	t.Setenv(stateDirEnv, "")

	_, err := openList(t.Context(), main)
	require.NoError(t, err)

	data, err := os.ReadFile(registry.Path())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"common_dir": "`+filepath.Join(gittest.Resolved(t, main), ".git")+`"`)
}

// Legacy never registers a repository while $WTM_STATE_DIR is set: the registry could not tell where any repository keeps its state.
func TestARepositoryWhoseStateIsMovedElsewhereIsNotRegistered(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	moved := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(moved, "config.toml"), nil, 0o644))
	t.Setenv(stateDirEnv, moved)

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

// Opening a repository with no wtm config once registered it, which a global stream's prune dropped at once, announcing it anew.
func TestOpeningARepositoryWithNoWtmConfigLeavesTheRegistryUnwritten(t *testing.T) {
	uninitialized := gittest.InitRepo(t)
	ownConfigDir(t)
	t.Setenv(stateDirEnv, "")

	_, err := openList(t.Context(), uninitialized)
	require.NoError(t, err)

	assert.NoFileExists(t, registry.Path())
}

func TestTheRegistryAnnouncesARepositoryItAddsAndOneItDrops(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	socket := servingBus(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	lines, err := bus.Bus{SocketPath: socket}.Subscribe(ctx, nil)
	require.NoError(t, err)
	repos := knownRepos(filepath.Join(t.TempDir(), "repos.json"), socket)
	repo := domain.Repo{Root: main, CommonDir: bus.Resolved(filepath.Join(main, ".git"))}

	repos.Register(t.Context(), repo)
	require.NoError(t, os.Remove(filepath.Join(main, ".git", "wtm", "config.toml")))
	kept, err := repos.Prune(t.Context())

	require.NoError(t, err)
	assert.Empty(t, kept)
	assert.Contains(t, string(<-lines), `"type":"repo.added"`)
	assert.Contains(t, string(<-lines), `"type":"repo.removed"`)
}

func TestARelativeRepoOfASurfaceIsReadFromTheCallsDirectory(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	t.Setenv(stateDirEnv, "")
	stream := api.BindStream[events.Request, events.Questions](events.Name, watchEvents(eventStream(streamParams{Socket: servingBus(t), Registry: filepath.Join(t.TempDir(), "repos.json")})))

	lines, response := watchedLines(t, stream, api.Call{Dir: filepath.Dir(main), Answers: []domain.Answered{{Path: "repo", Values: []string{filepath.Base(main)}}}}, 1)

	assert.Equal(t, http.StatusOK, response.Status, response.Envelope.Err)
	require.NotEmpty(t, lines)
	assert.Contains(t, lines[0], `"root":"`+gittest.Resolved(t, main)+`"`)
}

// The command line read a relative --repo from its working directory, never from $WTM_PROJECT_DIR, as legacy did and as a surface reads it from its call's directory.
func TestARelativeRepoOnTheCommandLineIsReadFromTheWorkingDirectory(t *testing.T) {
	main, elsewhere := configured(t), configured(t)
	ownConfigDir(t)
	t.Setenv(stateDirEnv, "")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	streams, _, _, _ := output.Test()
	stdout := &firstLine{stop: cancel}
	streams.Out.Writer = stdout
	env := cli.Env{Cwd: filepath.Dir(gittest.Resolved(t, main)), ProjectDir: elsewhere, Named: true}
	root := cli.NewRoot(version, streams, []*cobra.Command{eventscli.New(streams, env, eventStream(streamParams{Socket: servingBus(t), Registry: filepath.Join(t.TempDir(), "repos.json")}), commandLine)})

	code := cli.Execute(ctx, root, []string{"events", "--repo", filepath.Base(main), "--output", "json"})

	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), `"root":"`+gittest.Resolved(t, main)+`"`)
}

func TestTheEveryRepositoryStreamOfASurfaceIsACallNamingNoDirectory(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	t.Setenv(stateDirEnv, "")
	socket, registry := servingBus(t), filepath.Join(t.TempDir(), "repos.json")
	knownRepos(registry, socket).Register(t.Context(), domain.Repo{Root: gittest.Resolved(t, main), CommonDir: bus.Resolved(filepath.Join(main, ".git"))})
	stream := api.BindStream[events.Request, events.Questions](events.Name, watchEvents(eventStream(streamParams{Socket: socket, Registry: registry})))

	lines, response := watchedLines(t, stream, api.Call{}, 2)

	assert.Equal(t, http.StatusOK, response.Status, response.Envelope.Err)
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"type":"snapshot"`)
	assert.Contains(t, lines[0], `"root":"`+gittest.Resolved(t, main)+`"`)
	assert.JSONEq(t, `{"v":2,"type":"ready","ts":"x"}`, withTS(t, lines[1]))
}

// A repository read with a moved state would be read with the sandbox's config and records: the stream of every repository lists none while the state is moved.
func TestTheEveryRepositoryStreamListsNoRepositoryWhileTheStateIsMoved(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	t.Setenv(stateDirEnv, "")
	socket, registry := servingBus(t), filepath.Join(t.TempDir(), "repos.json")
	knownRepos(registry, socket).Register(t.Context(), domain.Repo{Root: gittest.Resolved(t, main), CommonDir: bus.Resolved(filepath.Join(main, ".git"))})
	require.FileExists(t, registry)
	moved := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(moved, "config.toml"), []byte("[worktrees]\nbase_path = \"../.trees\"\n"), 0o644))
	t.Setenv(stateDirEnv, moved)
	stream := api.BindStream[events.Request, events.Questions](events.Name, watchEvents(eventStream(streamParams{Socket: socket, Registry: registry})))

	lines, response := watchedLines(t, stream, api.Call{}, 1)

	assert.Equal(t, http.StatusOK, response.Status, response.Envelope.Err)
	require.Len(t, lines, 1)
	assert.JSONEq(t, `{"v":2,"type":"ready","ts":"x"}`, withTS(t, lines[0]))
}
