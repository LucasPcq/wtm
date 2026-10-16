// Package gitwt is a git worktree's lifecycle: added with its branch, removed with what the add created.
package gitwt

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/internal/infra/git"
)

type Params struct {
	Git      git.ReadWriter
	Path     string
	Checkout git.Checkout
}

// Add deletes a branch `worktree add -b` created before refusing the path: a saga never undoes a Do that failed.
func Add(ctx context.Context, params Params) error {
	add := git.AddWorktreeParams{Path: params.Path, Checkout: params.Checkout}
	created, ok := git.CreatedBranch(params.Checkout)
	if !ok {
		return params.Git.AddWorktree(ctx, add)
	}
	branch := git.LocalBranch{Name: created}
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

// Remove forces: the steps after the add put files in the worktree, and a branch born with it has no commit to lose.
func Remove(ctx context.Context, params Params) error {
	if err := params.Git.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: params.Path, Force: true}); err != nil {
		return err
	}
	created, ok := git.CreatedBranch(params.Checkout)
	if !ok {
		return nil
	}
	return params.Git.DeleteBranch(ctx, git.DeleteBranchParams{Name: created, Force: true})
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
