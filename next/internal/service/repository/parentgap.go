package repository

import "github.com/LucasPcq/wtm/next/internal/domain/worktree"

// A ParentGap is what a worktree's branch has past its parent, and what it lacks of it: Behind is nil for a root,
// which has no parent to fall behind and is counted against the base branch, and both are nil for a branch not
// counted.
type ParentGap struct {
	Ahead  *int
	Behind *int
}

// GapParams are the worktree to count and its repository and stack, its gap to the base branch when it was counted
// already, nil otherwise, and how to count it against a branch.
type GapParams struct {
	Repo     worktree.Repository
	Stack    worktree.Stack
	Worktree worktree.Worktree
	Base     *worktree.AheadBehind
	Count    func(against string) (worktree.AheadBehind, error)
}

// GapToParent counts the worktree's branch against its parent, the base branch for a root, as list, tree and status
// show it; against the base branch, a gap counted already is not counted again.
func GapToParent(p GapParams) (ParentGap, error) {
	against, counted := p.Repo.CountedAgainstParent(p.Stack, p.Worktree)
	if !counted {
		return ParentGap{}, nil
	}
	gap, err := p.gapTo(against)
	if err != nil {
		return ParentGap{}, err
	}

	found := ParentGap{Ahead: &gap.Ahead}
	if _, child := p.Stack.Parent(p.Worktree.Branch()); child {
		found.Behind = &gap.Behind
	}
	return found, nil
}

func (p GapParams) gapTo(against string) (worktree.AheadBehind, error) {
	if p.Base != nil && against == p.Repo.Config.Worktrees.BaseBranch {
		return *p.Base, nil
	}
	return p.Count(against)
}
