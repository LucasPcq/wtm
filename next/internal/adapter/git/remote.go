package git

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// FetchRef fetches the branch name from the remote, updating its remote-tracking
// branch only.
func (r Repo) FetchRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", name)
	return err
}

// FastForwardRef fetches the branch name from the remote straight into the
// local branch of that name; git refuses unless it is a fast-forward and the
// branch is checked out nowhere.
func (r Repo) FastForwardRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", r.Remote, "--", name+":"+name)
	return err
}

// MergeFFOnly fast-forwards the branch checked out at merge.Path onto
// merge.Onto, failing rather than merging when it cannot.
func (r Repo) MergeFFOnly(ctx context.Context, merge worktree.Merge) error {
	onto, err := r.refName(merge.Onto)
	if err != nil {
		return err
	}

	_, err = Repo{Dir: merge.Path, Remote: r.Remote}.git(ctx, "merge", "--ff-only", onto)
	return err
}
