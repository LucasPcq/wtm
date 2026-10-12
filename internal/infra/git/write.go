package git

import (
	"context"
	"errors"
)

// Writer is what a command changes in git. A new branch is born with its
// worktree, by AddWorktree: there is no separate branch creation to undo.
type Writer interface {
	AddWorktree(ctx context.Context, params AddWorktreeParams) error
	RemoveWorktree(ctx context.Context, params RemoveWorktreeParams) error
	DeleteBranch(ctx context.Context, params DeleteBranchParams) error
	Fetch(ctx context.Context) error
	FetchRef(ctx context.Context, branch string) error
	FastForwardRef(ctx context.Context, branch string) error
	MergeFFOnly(ctx context.Context, params MergeFFOnlyParams) error
}

//go-sumtype:decl Checkout

// Checkout is the branch a new worktree has checked out.
type Checkout interface{ checkout() }

// NewBranch never tracks From: a remote start point is not an upstream.
type NewBranch struct {
	Name string
	From Ref
}

type ExistingBranch struct {
	Name string
}

func (NewBranch) checkout()      {}
func (ExistingBranch) checkout() {}

type AddWorktreeParams struct {
	Path     string
	Checkout Checkout
}

type RemoveWorktreeParams struct {
	Path  string
	Force bool
}

type DeleteBranchParams struct {
	Name  string
	Force bool
}

type MergeFFOnlyParams struct {
	Worktree string
	Onto     Ref
}

var errNoCheckout = errors.New("no checkout given")

func (r Repo) AddWorktree(ctx context.Context, params AddWorktreeParams) error {
	switch checkout := params.Checkout.(type) {
	case NewBranch:
		from, err := r.refName(checkout.From)
		if err != nil {
			return err
		}
		_, err = r.git(ctx, "worktree", "add", "--no-track", "-b", checkout.Name, "--", params.Path, from)
		return err
	case ExistingBranch:
		_, err := r.git(ctx, "worktree", "add", "--", params.Path, checkout.Name)
		return err
	}
	return errNoCheckout
}

func (r Repo) RemoveWorktree(ctx context.Context, params RemoveWorktreeParams) error {
	args := []string{"worktree", "remove"}
	if params.Force {
		args = append(args, "--force")
	}
	_, err := r.git(ctx, append(args, "--", params.Path)...)
	return err
}

func (r Repo) DeleteBranch(ctx context.Context, params DeleteBranchParams) error {
	flag := "-d"
	if params.Force {
		flag = "-D"
	}
	_, err := r.git(ctx, "branch", flag, "--", params.Name)
	return err
}

// Fetch refreshes every branch of the remote, for a branch picker's refresh.
func (r Repo) Fetch(ctx context.Context) error {
	_, err := r.git(ctx, "fetch", r.Remote)
	return err
}

// FetchRef refreshes the remote branch alone; the local branch stays put.
func (r Repo) FetchRef(ctx context.Context, branch string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", branch)
	return err
}

// FastForwardRef moves a local branch checked out nowhere to its remote tip.
// git refuses anything but a fast-forward, and a branch a worktree has checked
// out: that one moves by MergeFFOnly, inside its worktree.
func (r Repo) FastForwardRef(ctx context.Context, branch string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", branch+":"+branch)
	return err
}

func (r Repo) MergeFFOnly(ctx context.Context, params MergeFFOnlyParams) error {
	onto, err := r.refName(params.Onto)
	if err != nil {
		return err
	}
	_, err = run(ctx, []string{"-C", params.Worktree, "merge", "--ff-only", onto})
	return err
}
