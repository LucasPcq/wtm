package worktree_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/rules/worktree"
)

func TestARebaseInProgressNamesTheBranchItMoves(t *testing.T) {
	assert.Equal(t, "feature", worktree.RebasingBranch(worktree.RebasingParams{HeadNames: []string{"refs/heads/feature\n"}}))
	assert.Equal(t, "feature", worktree.RebasingBranch(worktree.RebasingParams{HeadNames: []string{"", "refs/heads/feature"}}))
}

func TestAHeadMerelyDetachedNamesNoBranch(t *testing.T) {
	assert.Empty(t, worktree.RebasingBranch(worktree.RebasingParams{}))
	assert.Empty(t, worktree.RebasingBranch(worktree.RebasingParams{HeadNames: []string{"\n"}}))
}

func TestAWorktreeWhoseDirectoryIsGoneIsMerelyDetached(t *testing.T) {
	assert.Empty(t, worktree.RebasingBranch(worktree.RebasingParams{DirGone: true, HeadNames: []string{"refs/heads/feature"}}))
}
