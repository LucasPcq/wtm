// Package gitwt is a git worktree's lifecycle: added with its branch, removed with what the add created.
package gitwt

import (
	"context"

	"github.com/LucasPcq/wtm/internal/infra/git"
)

// Params are one worktree's: Add creates it, Remove given the same Params
// undoes it, so the two are the Do and the Undo of a saga's worktree step.
type Params struct {
	Git      git.Writer
	Path     string
	Checkout git.Checkout
}

// Add checks no guard: what may be added was decided from a snapshot
// read under the operation's locks.
func Add(ctx context.Context, params Params) error {
	return params.Git.AddWorktree(ctx, git.AddWorktreeParams{Path: params.Path, Checkout: params.Checkout})
}

// Remove forces: an undone worktree holds what the steps after the add put in
// it, and a branch born with it has no commit of its own to lose. A branch
// that existed before the add is never deleted.
func Remove(ctx context.Context, params Params) error {
	if err := params.Git.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: params.Path, Force: true}); err != nil {
		return err
	}
	created, ok := params.Checkout.(git.NewBranch)
	if !ok {
		return nil
	}
	return params.Git.DeleteBranch(ctx, git.DeleteBranchParams{Name: created.Name, Force: true})
}
