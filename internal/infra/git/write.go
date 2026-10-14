package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Writer is what a command changes in git. A new branch is born with its
// worktree, by AddWorktree: there is no separate branch creation to undo.
type Writer interface {
	AddWorktree(ctx context.Context, params AddWorktreeParams) error
	RemoveWorktree(ctx context.Context, params RemoveWorktreeParams) error
	DeleteBranch(ctx context.Context, params DeleteBranchParams) error
	Fetch(ctx context.Context) error
	FetchRef(ctx context.Context, branch string) error
	FastForwardRef(ctx context.Context, branch string) (Move, error)
	MergeFFOnly(ctx context.Context, params MergeFFOnlyParams) (Move, error)
}

//go-sumtype:decl Checkout

// Checkout is the branch a new worktree has checked out.
type Checkout interface{ checkout() }

// NewBranch never tracks From: a remote start point is not an upstream.
type NewBranch struct {
	Name string
	From Ref
}

// ExistingBranch is a local branch: git would turn a name only the remote has
// into a new branch tracking it, which the add refuses instead.
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

// Move is where a fast-forward took a branch; From is empty for a branch it
// created, and equals To when there was nothing to bring.
type Move struct {
	From string
	To   string
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
		local, err := r.RefExists(ctx, LocalBranch(checkout))
		if err != nil {
			return err
		}
		if !local {
			return fmt.Errorf("%w: %s", ErrNoLocalBranch, checkout.Name)
		}
		_, err = r.git(ctx, "worktree", "add", "--", params.Path, checkout.Name)
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

// FastForwardRef moves a local branch checked out nowhere to its remote tip,
// creating it when it is absent. git refuses anything but a fast-forward, and a
// branch a worktree has checked out: that one moves by MergeFFOnly, inside its
// worktree.
func (r Repo) FastForwardRef(ctx context.Context, branch string) (Move, error) {
	local := LocalBranch{Name: branch}
	from, err := r.tipIfAny(ctx, local)
	if err != nil {
		return Move{}, err
	}
	if _, err := r.git(ctx, "fetch", r.Remote, "--", branch+":"+branch); err != nil {
		return Move{}, err
	}
	to, err := r.Tip(ctx, local)
	if err != nil {
		return Move{}, err
	}
	return Move{From: from, To: to}, nil
}

func (r Repo) MergeFFOnly(ctx context.Context, params MergeFFOnlyParams) (Move, error) {
	onto, err := r.refName(params.Onto)
	if err != nil {
		return Move{}, err
	}
	worktree := Repo{Dir: params.Worktree, Remote: r.Remote}
	from, err := worktree.head(ctx)
	if err != nil {
		return Move{}, err
	}
	if _, err := worktree.git(ctx, "merge", "--ff-only", onto); err != nil {
		return Move{}, err
	}
	to, err := worktree.head(ctx)
	if err != nil {
		return Move{}, err
	}
	return Move{From: from, To: to}, nil
}

func (r Repo) tipIfAny(ctx context.Context, ref Ref) (string, error) {
	exists, err := r.RefExists(ctx, ref)
	if err != nil || !exists {
		return "", err
	}
	return r.Tip(ctx, ref)
}

func (r Repo) head(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(string(out)), err
}
