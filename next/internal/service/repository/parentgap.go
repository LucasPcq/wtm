package repository

import "github.com/LucasPcq/wtm/next/internal/domain/worktree"

// A ParentGap is what a worktree's branch has past its parent, and what it lacks of it: Behind is nil for a root,
// which has no parent to fall behind and is counted against the base branch, and both are nil for a branch not
// counted.
type ParentGap struct {
	Ahead  *int
	Behind *int
}

// GapToParent counts w's branch against its parent, the base branch for a root, with count, as list and tree show
// it.
func GapToParent(repo worktree.Repository, stack worktree.Stack, w worktree.Worktree, count func(against string) (worktree.AheadBehind, error)) (ParentGap, error) {
	against, counted := repo.CountedAgainstParent(stack, w)
	if !counted {
		return ParentGap{}, nil
	}
	gap, err := count(against)
	if err != nil {
		return ParentGap{}, err
	}

	found := ParentGap{Ahead: &gap.Ahead}
	if _, child := stack.Parent(w.Branch()); child {
		found.Behind = &gap.Behind
	}
	return found, nil
}
