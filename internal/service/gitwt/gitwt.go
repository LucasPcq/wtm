// Package gitwt is a git worktree's lifecycle: added with its branch, removed with what the add created.
package gitwt

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/internal/infra/git"
)

// Params are one worktree's: Add creates it, Remove given the same Params
// undoes it, so the two are the Do and the Undo of a saga's worktree step.
type Params struct {
	Git      git.ReadWriter
	Path     string
	Checkout git.Checkout
}

// Add checks no guard: what may be added was decided from a snapshot read
// under the operation's locks. A saga never undoes a Do that failed, so an add
// git refuses after creating the branch deletes it here.
func Add(ctx context.Context, params Params) error {
	add := git.AddWorktreeParams{Path: params.Path, Checkout: params.Checkout}
	created, ok := params.Checkout.(git.NewBranch)
	if !ok {
		return params.Git.AddWorktree(ctx, add)
	}
	branch := git.LocalBranch{Name: created.Name}
	existed, err := params.Git.RefExists(ctx, branch)
	if err != nil {
		return err
	}
	err = params.Git.AddWorktree(ctx, add)
	if err == nil || existed {
		return err
	}
	return errors.Join(err, dropLeft(ctx, dropLeftParams{Git: params.Git, Branch: branch}))
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

type dropLeftParams struct {
	Git    git.ReadWriter
	Branch git.LocalBranch
}

func dropLeft(ctx context.Context, params dropLeftParams) error {
	left, err := params.Git.RefExists(ctx, params.Branch)
	if err != nil || !left {
		return err
	}
	return params.Git.DeleteBranch(ctx, git.DeleteBranchParams{Name: params.Branch.Name, Force: true})
}
