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
	require.NoError(t, fake.MergeFFOnly(t.Context(), git.MergeFFOnlyParams{Worktree: "/repo", Onto: git.RemoteBranch{Name: "main"}}))

	assert.Equal(t, []string{"FetchRef main", "Worktrees ", "MergeFFOnly /repo"}, fake.Calls())
}

func TestAFakeCountsWhatItIsGiven(t *testing.T) {
	pair := git.AheadBehindParams{Of: git.LocalBranch{Name: "main"}, Against: git.RemoteBranch{Name: "main"}}
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Remote: []string{"main"}, AheadBehind: map[git.AheadBehindParams]git.AheadBehind{pair: {Ahead: 1, Behind: 3}}})

	counts, err := fake.AheadBehind(t.Context(), pair)

	require.NoError(t, err)
	assert.Equal(t, git.AheadBehind{Ahead: 1, Behind: 3}, counts)
}

func TestAFakeFastForwardsABranchCheckedOutNowhere(t *testing.T) {
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Local: []string{"feature"}, Remote: []string{"main", "feature"}})

	require.NoError(t, fake.FastForwardRef(t.Context(), "feature"))
	require.NoError(t, fake.Fetch(t.Context()))
	require.Error(t, fake.FastForwardRef(t.Context(), "gone"))
	require.Error(t, fake.MergeFFOnly(t.Context(), git.MergeFFOnlyParams{Worktree: "/elsewhere", Onto: git.RemoteBranch{Name: "main"}}))
}
