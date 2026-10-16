package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWorktreeIsOnTheBranchItHasCheckedOutOrIsRebasing(t *testing.T) {
	assert.Equal(t, "feat/a", Worktree{State: OnBranch{Branch: "feat/a"}}.Branch())
	assert.Equal(t, "feat/b", Worktree{State: Rebasing{Branch: "feat/b"}}.Branch())
	assert.Empty(t, Worktree{State: Detached{}}.Branch())
	assert.Empty(t, Worktree{State: Bare{}}.Branch())
}
