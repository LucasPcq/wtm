package repository_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
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

	repo, err := repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Fail: map[string]error{"LoadRun": runErr}}), Git: git, MainPath: "/repo/main"}.Observe(t.Context(), "greet")

	require.NoError(t, err)
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	assert.Equal(t, worktree.Repository{MainPath: "/repo/main", Config: config, RunErr: runErr, Snapshot: snapshot}, repo)
}

func TestARepositoryWithoutConfigIsNotObservedFurther(t *testing.T) {
	config := configtest.NewFake(configtest.FakeParams{})

	_, err := repository.Service{Config: config, Git: gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"}), Init: &domain.FollowUp{Command: "setup"}}.Observe(t.Context(), "greet")

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	assert.Equal(t, "greet.config_not_found", refused.Code)
	assert.Equal(t, "no wtm config found", refused.Message)
	assert.Equal(t, &domain.FollowUp{Command: "setup", Code: "repository.init", Message: "set wtm up in this repository"}, refused.FollowUp)
	assert.ErrorIs(t, err, project.ErrConfigNotFound)
	assert.NotContains(t, config.Calls(), "LoadRun")
}

func TestARepositoryWithAnInvalidConfigIsRefusedSayingWhy(t *testing.T) {
	reason := errors.New("base_path must be relative to the repository: /tmp/trees")
	config := configtest.NewFake(configtest.FakeParams{Fail: map[string]error{"Load": &project.InvalidConfigError{Reason: reason}}})

	_, err := repository.Service{Config: config, Git: gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main"})}.Observe(t.Context(), "greet")

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	assert.Equal(t, "greet.config_invalid", refused.Code)
	assert.Equal(t, "invalid config.toml: base_path must be relative to the repository: /tmp/trees", refused.Text())
	assert.ErrorIs(t, err, reason)
	assert.NotContains(t, config.Calls(), "LoadRun")
}
