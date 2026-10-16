package gittest

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFakeMethodGivenAnErrorFailsWithoutRunning(t *testing.T) {
	offline := errors.New("offline")
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}, Fail: map[string]error{"FetchRef": offline, "AddWorktree": offline}})

	fetchErr := fake.FetchRef(t.Context(), "main")
	addErr := fake.AddWorktree(t.Context(), worktree.Addition{Path: "/topic", Checkout: worktree.NewBranch{Name: "topic", From: branch.LocalBranch{Name: "main"}}})

	require.ErrorIs(t, fetchErr, offline)
	require.ErrorIs(t, addErr, offline)
	branches, err := fake.Branches(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, Names(branches.Local))
}

func TestAFakeRecordsEveryCallInOrder(t *testing.T) {
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}})

	require.NoError(t, fake.FetchRef(t.Context(), "main"))
	_, err := fake.Worktrees(t.Context())
	require.NoError(t, err)
	_, err = fake.MergeFFOnly(t.Context(), worktree.Merge{Path: "/repo", Onto: branch.RemoteBranch{Name: "main"}})
	require.NoError(t, err)

	assert.Equal(t, []string{"FetchRef main", "Worktrees ", "MergeFFOnly /repo"}, fake.Calls())
}

func TestAFakeCommitIsItsName(t *testing.T) {
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Commits: map[branch.Ref][]string{branch.LocalBranch{Name: "main"}: {"first", "second"}}})

	branches, err := fake.Branches(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []branch.Branch{{Name: "main", Tip: "second"}}, branches.Local)
}

func TestAFakeMergeOnADetachedHeadIsNotModelled(t *testing.T) {
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}, Worktrees: []worktree.Worktree{{Path: "/wt/detached", State: worktree.Detached{}}}})

	_, err := fake.MergeFFOnly(t.Context(), worktree.Merge{Path: "/wt/detached", Onto: branch.RemoteBranch{Name: "main"}})

	require.Error(t, err)
}

func TestAFakeAddThatCannotDropTheBranchItLeftSaysSo(t *testing.T) {
	refused := errors.New("refused")
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Folders: []string{"/wt/topic"}, Fail: map[string]error{"DeleteBranch": refused}})

	err := fake.AddWorktree(t.Context(), worktree.Addition{Path: "/wt/topic", Checkout: worktree.NewBranch{Name: "topic", From: branch.LocalBranch{Name: "main"}}})

	require.ErrorIs(t, err, refused)
	assert.Contains(t, fake.Calls(), "DeleteBranch topic")
}
