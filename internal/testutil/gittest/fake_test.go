package gittest_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// The fake's agreement with git is tested in internal/infra/git
// (contract_test.go); here, what only the fake does.

func TestAFakeMethodGivenAnErrorFailsWithoutRunning(t *testing.T) {
	offline := errors.New("offline")
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}, Fail: map[string]error{"Fetch": offline, "AddWorktree": offline}})

	fetchErr := fake.Fetch(t.Context())
	addErr := fake.AddWorktree(t.Context(), git.AddWorktreeParams{Path: "/topic", Checkout: git.NewBranch{Name: "topic", From: git.LocalBranch{Name: "main"}}})

	require.ErrorIs(t, fetchErr, offline)
	require.ErrorIs(t, addErr, offline)
	exists, err := fake.RefExists(t.Context(), git.LocalBranch{Name: "topic"})
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestAFakeRecordsEveryCallInOrder(t *testing.T) {
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}})

	require.NoError(t, fake.FetchRef(t.Context(), "main"))
	_, err := fake.Worktrees(t.Context())
	require.NoError(t, err)
	_, err = fake.MergeFFOnly(t.Context(), git.MergeFFOnlyParams{Worktree: "/repo", Onto: git.RemoteBranch{Name: "main"}})
	require.NoError(t, err)

	assert.Equal(t, []string{"FetchRef main", "Worktrees ", "MergeFFOnly /repo"}, fake.Calls())
}

func TestAFakeCommitIsItsName(t *testing.T) {
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Commits: map[git.Ref][]string{git.LocalBranch{Name: "main"}: {"first", "second"}}})

	tip, err := fake.Tip(t.Context(), git.LocalBranch{Name: "main"})
	main, mainErr := fake.MainWorktree(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "second", tip)
	require.NoError(t, mainErr)
	assert.Equal(t, "second", main.Head)
}

func TestAFakeSeenFromALinkedWorktreeMergesIntoItsHead(t *testing.T) {
	seed := func(dir string) gittest.FakeParams {
		return gittest.FakeParams{
			Main:      "/repo",
			Dir:       dir,
			Branch:    "main",
			Local:     []string{"part"},
			Worktrees: []git.Worktree{{Path: "/wt/feature", State: git.OnBranch{Branch: "feature"}}},
			Commits:   map[git.Ref][]string{git.LocalBranch{Name: "feature"}: {"work"}, git.LocalBranch{Name: "part"}: {"work"}},
		}
	}
	fromFeature := gittest.NewFake(seed("/wt/feature/sub"))
	fromMain := gittest.NewFake(seed(""))

	top, err := fromFeature.Toplevel(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "/wt/feature", top)
	require.NoError(t, fromFeature.DeleteBranch(t.Context(), git.DeleteBranchParams{Name: "part"}))
	require.Error(t, fromMain.DeleteBranch(t.Context(), git.DeleteBranchParams{Name: "part"}))
}

func TestAFakeMergeOnADetachedHeadIsNotModelled(t *testing.T) {
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}, Worktrees: []git.Worktree{{Path: "/wt/detached", State: git.Detached{}}}})

	_, err := fake.MergeFFOnly(t.Context(), git.MergeFFOnlyParams{Worktree: "/wt/detached", Onto: git.RemoteBranch{Name: "main"}})

	require.Error(t, err)
}
