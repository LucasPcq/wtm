package repository_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
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

	repo, err := repository.Service{Config: configured{config: config, runErr: runErr, loadRun: &loadRun}, Git: git, MainPath: "/repo/main"}.Observe(t.Context(), "greet")

	require.NoError(t, err)
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	assert.Equal(t, worktree.Repository{MainPath: "/repo/main", Config: config, RunErr: runErr, Snapshot: snapshot}, repo)
}

func TestARepositoryWithoutConfigIsNotObservedFurther(t *testing.T) {
	loadRun := 0

	_, err := repository.Service{Config: configured{err: project.ErrConfigNotFound, loadRun: &loadRun}, Git: gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"})}.Observe(t.Context(), "greet")

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	assert.Equal(t, "greet.config_not_found", refused.Code)
	assert.ErrorIs(t, err, project.ErrConfigNotFound)
	assert.Zero(t, loadRun)
}

func TestARepositoryWithAnInvalidConfigIsRefusedSayingWhy(t *testing.T) {
	loadRun := 0
	reason := errors.New("base_path must be relative to the repository: /tmp/trees")

	_, err := repository.Service{Config: configured{err: &project.InvalidConfigError{Reason: reason}, loadRun: &loadRun}, Git: gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"})}.Observe(t.Context(), "greet")

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	assert.Equal(t, "greet.config_invalid", refused.Code)
	assert.Equal(t, "invalid config.toml: base_path must be relative to the repository: /tmp/trees", refused.Text())
	assert.ErrorIs(t, err, reason)
	assert.Zero(t, loadRun)
}
