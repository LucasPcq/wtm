package repository_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheRepositoryObservedIsTheConfigTheSnapshotAndTheRunWithItsError(t *testing.T) {
	git := gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"})
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees"}}
	runErr := errors.New("bad run.toml")
	loadRun := 0

	repo, err := repository.Observe(t.Context(), repository.ObserveParams{Config: configured{config: config, runErr: runErr, loadRun: &loadRun}, Git: git, MainPath: "/repo/main"})

	require.NoError(t, err)
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	assert.Equal(t, worktree.Repository{MainPath: "/repo/main", Config: config, RunErr: runErr, Snapshot: snapshot}, repo)
}

func TestARepositoryWithoutConfigIsNotObservedFurther(t *testing.T) {
	loadRun := 0

	_, err := repository.Observe(t.Context(), repository.ObserveParams{Config: configured{err: project.ErrConfigNotFound, loadRun: &loadRun}, Git: gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"})})

	assert.ErrorIs(t, err, project.ErrConfigNotFound)
	assert.Zero(t, loadRun)
}
