package repository_test

import (
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// linkedAt is a linked worktree at /repo/.trees/<name>, on the branch name.
func linkedAt(name string) worktree.Worktree {
	return worktree.Worktree{Path: "/repo/.trees/" + name, State: worktree.OnBranch{Branch: name}}
}
