package provision_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheRepositoryObservedIsTheConfigTheSnapshotAndTheRunWithItsError(t *testing.T) {
	git := gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"})
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees"}}
	runErr := errors.New("bad run.toml")
	loadRun := 0

	repo, err := provision.Observe(t.Context(), provision.ObserveParams{Config: configured{config: config, runErr: runErr, loadRun: &loadRun}, Git: git, MainPath: "/repo/main"})

	require.NoError(t, err)
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	assert.Equal(t, worktree.Repository{MainPath: "/repo/main", Config: config, RunErr: runErr, Snapshot: snapshot}, repo)
}

func TestARepositoryWithoutConfigIsNotObservedFurther(t *testing.T) {
	loadRun := 0

	_, err := provision.Observe(t.Context(), provision.ObserveParams{Config: configured{err: project.ErrConfigNotFound, loadRun: &loadRun}, Git: gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"})})

	assert.ErrorIs(t, err, project.ErrConfigNotFound)
	assert.Zero(t, loadRun)
}

func TestTheProjectProvisionedIsTheOneObserved(t *testing.T) {
	runErr := errors.New("bad run.toml")
	repo := worktree.Repository{
		MainPath: "/repo/main",
		Config:   project.Config{Env: project.Env{Files: []project.EnvFile{{Target: ".env"}}}, OnCreate: []project.Hook{{Cmd: "make"}}},
		Global:   project.Global{Proxy: project.Proxy{Port: 8080}},
		Run:      project.Run{Isolation: project.Verbatim},
		RunErr:   runErr,
	}

	assert.Equal(t, provision.Project{
		MainPath: "/repo/main",
		StateDir: "/state",
		EnvFiles: []project.EnvFile{{Target: ".env"}},
		OnCreate: []project.Hook{{Cmd: "make"}},
		Run:      project.Run{Isolation: project.Verbatim},
		RunErr:   runErr,
		Global:   project.Global{Proxy: project.Proxy{Port: 8080}},
	}, provision.ProjectOf(repo, "/state"))
}
