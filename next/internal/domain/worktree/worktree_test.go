package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWorktreeIsOnTheBranchItHasCheckedOutOrIsRebasingOrBisecting(t *testing.T) {
	assert.Equal(t, "feat/a", Worktree{State: OnBranch{Branch: "feat/a"}}.Branch())
	assert.Equal(t, "feat/b", Worktree{State: Rebasing{Branch: "feat/b"}}.Branch())
	assert.Equal(t, "feat/c", Worktree{State: Bisecting{Branch: "feat/c"}}.Branch())
	assert.Empty(t, Worktree{State: Detached{}}.Branch())
	assert.Empty(t, Worktree{State: Bare{}}.Branch())
}

func TestOnlyANewOrATrackingCheckoutCreatesItsBranch(t *testing.T) {
	created, ok := CreatedBranch(NewBranch{Name: "topic", From: LocalBranch{Name: "main"}})
	assert.True(t, ok)
	assert.Equal(t, "topic", created)

	created, ok = CreatedBranch(TrackingBranch{Name: "remote-only"})
	assert.True(t, ok)
	assert.Equal(t, "remote-only", created)

	_, ok = CreatedBranch(ExistingBranch{Name: "feature"})
	assert.False(t, ok)
}

func TestTheFreeOrdinalIsTheSmallestNoWorktreeHoldsAfterTheMainCheckout(t *testing.T) {
	assert.Equal(t, 1, FreeOrdinal(nil))
	assert.Equal(t, 1, FreeOrdinal([]int{0, 0}))
	assert.Equal(t, 3, FreeOrdinal([]int{2, 1, 2}))
	assert.Equal(t, 2, FreeOrdinal([]int{1, 3}))
}
