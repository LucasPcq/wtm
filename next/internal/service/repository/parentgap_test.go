package repository_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

func TestAChildIsCountedAgainstItsParentBothWaysARootAgainstTheBaseAheadOnly(t *testing.T) {
	git := gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main", Worktrees: []worktree.Worktree{linkedAt("feat/a"), linkedAt("feat/b")}})
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	repo := worktree.Repository{Snapshot: snapshot, Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}}
	stack := worktree.StackOf(snapshot.Worktrees, map[string]worktree.Meta{"feat/b": {SourceBranch: "feat/a"}}, "main")
	var asked []string
	count := func(against string) (worktree.AheadBehind, error) {
		asked = append(asked, against)
		return worktree.AheadBehind{Ahead: 2, Behind: 3}, nil
	}

	root, err := repository.GapToParent(repo, stack, linkedAt("feat/a"), count)
	require.NoError(t, err)
	child, err := repository.GapToParent(repo, stack, linkedAt("feat/b"), count)
	require.NoError(t, err)

	assert.Equal(t, repository.ParentGap{Ahead: new(2)}, root)
	assert.Equal(t, repository.ParentGap{Ahead: new(2), Behind: new(3)}, child)
	assert.Equal(t, []string{"main", "feat/a"}, asked)
}

func TestAGapThatCannotBeCountedIsAnError(t *testing.T) {
	git := gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main", Worktrees: []worktree.Worktree{linkedAt("feat/a")}})
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	repo := worktree.Repository{Snapshot: snapshot, Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}}
	boom := errors.New("rev-list failed")

	gap, err := repository.GapToParent(repo, worktree.StackOf(snapshot.Worktrees, nil, "main"), linkedAt("feat/a"), func(string) (worktree.AheadBehind, error) {
		return worktree.AheadBehind{}, boom
	})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, repository.ParentGap{}, gap)
}
