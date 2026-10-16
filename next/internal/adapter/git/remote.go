package git

import (
	"context"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

func (r Repo) FetchRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", name)
	return err
}

func (r Repo) FastForwardRef(ctx context.Context, name string) (branch.Move, error) {
	local := branch.LocalBranch{Name: name}
	from, err := r.tipIfAny(ctx, local)
	if err != nil {
		return branch.Move{}, err
	}
	if _, err := r.git(ctx, "fetch", r.Remote, "--", name+":"+name); err != nil {
		return branch.Move{}, err
	}
	to, err := r.tip(ctx, local)
	if err != nil {
		return branch.Move{}, err
	}
	return branch.Move{From: from, To: to}, nil
}

func (r Repo) MergeFFOnly(ctx context.Context, merge worktree.Merge) (branch.Move, error) {
	onto, err := r.refName(merge.Onto)
	if err != nil {
		return branch.Move{}, err
	}
	in := Repo{Dir: merge.Path, Remote: r.Remote}
	from, err := in.head(ctx)
	if err != nil {
		return branch.Move{}, err
	}
	if _, err := in.git(ctx, "merge", "--ff-only", onto); err != nil {
		return branch.Move{}, err
	}
	to, err := in.head(ctx)
	if err != nil {
		return branch.Move{}, err
	}
	return branch.Move{From: from, To: to}, nil
}

func (r Repo) tipIfAny(ctx context.Context, ref branch.Ref) (string, error) {
	exists, err := r.refExists(ctx, ref)
	if err != nil || !exists {
		return "", err
	}
	return r.tip(ctx, ref)
}

func (r Repo) head(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(string(out)), err
}
