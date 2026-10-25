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

	root, err := repository.GapToParent(repository.GapParams{Repo: repo, Stack: stack, Worktree: linkedAt("feat/a"), Count: count})
	require.NoError(t, err)
	child, err := repository.GapToParent(repository.GapParams{Repo: repo, Stack: stack, Worktree: linkedAt("feat/b"), Count: count})
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

	gap, err := repository.GapToParent(repository.GapParams{Repo: repo, Stack: worktree.StackOf(snapshot.Worktrees, nil, "main"), Worktree: linkedAt("feat/a"), Count: func(string) (worktree.AheadBehind, error) {
		return worktree.AheadBehind{}, boom
	}})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, repository.ParentGap{}, gap)
}

func TestAGapToTheBaseBranchCountedAlreadyIsNotCountedAgain(t *testing.T) {
	git := gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main", Worktrees: []worktree.Worktree{linkedAt("feat/a"), linkedAt("feat/b")}})
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	repo := worktree.Repository{Snapshot: snapshot, Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}}
	stack := worktree.StackOf(snapshot.Worktrees, map[string]worktree.Meta{"feat/a": {SourceBranch: "main"}, "feat/b": {SourceBranch: "feat/a"}}, "main")
	var asked []string
	count := func(against string) (worktree.AheadBehind, error) {
		asked = append(asked, against)
		return worktree.AheadBehind{Ahead: 5, Behind: 6}, nil
	}
	base := &worktree.AheadBehind{Ahead: 1, Behind: 4}

	onBase, err := repository.GapToParent(repository.GapParams{Repo: repo, Stack: stack, Worktree: linkedAt("feat/a"), Base: base, Count: count})
	require.NoError(t, err)
	onParent, err := repository.GapToParent(repository.GapParams{Repo: repo, Stack: stack, Worktree: linkedAt("feat/b"), Base: base, Count: count})
	require.NoError(t, err)

	assert.Equal(t, repository.ParentGap{Ahead: new(1), Behind: new(4)}, onBase)
	assert.Equal(t, repository.ParentGap{Ahead: new(5), Behind: new(6)}, onParent)
	assert.Equal(t, []string{"feat/a"}, asked, "a child of the base branch reuses its gap")
}
