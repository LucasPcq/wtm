package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

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

type TrackingBranch struct {
	Name string
}

func (NewBranch) checkout()      {}
func (ExistingBranch) checkout() {}
func (TrackingBranch) checkout() {}

func CreatedBranch(checkout Checkout) (string, bool) {
	switch c := checkout.(type) {
	case NewBranch:
		return c.Name, true
	case TrackingBranch:
		return c.Name, true
	case ExistingBranch:
	}
	return "", false
}

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
	case TrackingBranch:
		_, err := r.git(ctx, "worktree", "add", "--track", "-b", checkout.Name, "--", params.Path, r.remotePrefix()+checkout.Name)
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

func (r Repo) Fetch(ctx context.Context) error {
	_, err := r.git(ctx, "fetch", r.Remote)
	return err
}

func (r Repo) FetchRef(ctx context.Context, branch string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", branch)
	return err
}

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
