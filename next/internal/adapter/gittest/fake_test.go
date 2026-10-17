package gittest

import (
	"errors"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFakeMethodGivenAnErrorFailsWithoutRunning(t *testing.T) {
	offline := errors.New("offline")
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}, Fail: map[string]error{"FetchRef": offline, "AddWorktree": offline}})

	fetchErr := fake.FetchRef(t.Context(), "main")
	addErr := fake.AddWorktree(t.Context(), worktree.Addition{Path: "/topic", Checkout: worktree.NewBranch{Name: "topic", From: worktree.LocalBranch{Name: "main"}}})

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
	require.NoError(t, fake.MergeFFOnly(t.Context(), worktree.Merge{Path: "/repo", Onto: worktree.RemoteBranch{Name: "main"}}))

	assert.Equal(t, []string{"FetchRef main", "Worktrees ", "MergeFFOnly /repo"}, fake.Calls())
}

func TestAFakeCommitIsItsName(t *testing.T) {
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Commits: map[worktree.Ref][]string{worktree.LocalBranch{Name: "main"}: {"first", "second"}}})

	branches, err := fake.Branches(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []worktree.Branch{{Name: "main", Tip: "second"}}, branches.Local)
}

func TestAFakeMergeOnADetachedHeadIsNotModelled(t *testing.T) {
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}, Worktrees: []worktree.Worktree{{Path: "/wt/detached", State: worktree.Detached{}}}})

	err := fake.MergeFFOnly(t.Context(), worktree.Merge{Path: "/wt/detached", Onto: worktree.RemoteBranch{Name: "main"}})

	require.Error(t, err)
}

func TestAFakeAddThatCannotDropTheBranchItLeftSaysSo(t *testing.T) {
	refused := errors.New("refused")
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Folders: []string{"/wt/topic"}, Fail: map[string]error{"DeleteBranch": refused}})

	err := fake.AddWorktree(t.Context(), worktree.Addition{Path: "/wt/topic", Checkout: worktree.NewBranch{Name: "topic", From: worktree.LocalBranch{Name: "main"}}})

	require.ErrorIs(t, err, refused)
	assert.Contains(t, fake.Calls(), "DeleteBranch topic")
}

func TestAFakeMoveTakesTheChangesFromTheSourceToTheTarget(t *testing.T) {
	moved := []worktree.Change{{Path: "a.go", Status: worktree.ChangeModified}}
	left := worktree.Change{Path: "b.go", Status: worktree.ChangeUntracked}
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Changes: map[string][]worktree.Change{"/repo": append(slices.Clone(moved), left)}})

	result, err := fake.Move(t.Context(), worktree.Move{Source: "/repo", Target: "/wt/split", Changes: moved})

	require.NoError(t, err)
	assert.Equal(t, worktree.Moved{}, result)
	source, _ := fake.Changes(t.Context(), "/repo")
	target, _ := fake.Changes(t.Context(), "/wt/split")
	assert.Equal(t, []worktree.Change{left}, source)
	assert.Equal(t, moved, target)
	assert.Equal(t, []worktree.Move{{Source: "/repo", Target: "/wt/split", Changes: moved}}, fake.Moves())
}

func TestAFakeConflictRefusesAMoveUnlessResolvedAndAResolvedOneKeepsTheSource(t *testing.T) {
	changes := []worktree.Change{{Path: "a.go", Status: worktree.ChangeModified}, {Path: "b.go", Status: worktree.ChangeModified}}
	fake := NewFake(FakeParams{Main: "/repo", Branch: "main", Changes: map[string][]worktree.Change{"/repo": changes}, Conflicts: map[string]worktree.ConflictKind{"a.go": worktree.ConflictModified}})
	move := worktree.Move{Source: "/repo", Target: "/wt/split", Changes: changes, TargetLabel: "split"}

	conflicts, err := fake.Conflicts(t.Context(), worktree.ConflictCheck{Source: "/repo", TargetRef: "split", Changes: changes})
	require.NoError(t, err)
	_, refused := fake.Move(t.Context(), move)
	move.Resolve = true
	resolved, err := fake.Move(t.Context(), move)

	assert.Equal(t, []worktree.Conflict{{Path: "a.go", Kind: worktree.ConflictModified}}, conflicts)
	require.ErrorContains(t, refused, `a.go was also modified in "split"`)
	require.NoError(t, err)
	assert.Equal(t, worktree.Moved{Merged: []string{"a.go"}, Kept: true}, resolved)
	source, _ := fake.Changes(t.Context(), "/repo")
	assert.Equal(t, changes, source)
}
