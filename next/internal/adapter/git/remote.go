package git

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

func (r Repo) FetchRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", name)
	return err
}

func (r Repo) FastForwardRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", name+":"+name)
	return err
}

func (r Repo) MergeFFOnly(ctx context.Context, merge worktree.Merge) error {
	onto, err := r.refName(merge.Onto)
	if err != nil {
		return err
	}
	_, err = Repo{Dir: merge.Path, Remote: r.Remote}.git(ctx, "merge", "--ff-only", onto)
	return err
}
