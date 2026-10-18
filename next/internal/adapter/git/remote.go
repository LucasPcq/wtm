package git

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Fetch fetches every branch of the remote, updating its remote-tracking
// branches only.
func (r Repo) Fetch(ctx context.Context) error {
	_, err := r.git(ctx, "fetch", "--quiet", origin)
	return err
}

// FetchRef fetches the branch name from the remote, updating its remote-tracking
// branch only. Branches are named in full so a tag of the same name, or a
// branch named tags/..., is never fetched instead.
func (r Repo) FetchRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", origin, "--", headsPrefix+name)
	return err
}

// FastForwardRef fetches the branch name from the remote straight into the
// local branch of that name; git refuses unless it is a fast-forward and the
// branch is checked out nowhere.
func (r Repo) FastForwardRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", origin, "--", headsPrefix+name+":"+headsPrefix+name)
	return err
}

// MergeFFOnly fast-forwards the branch checked out at merge.Path onto
// merge.Onto, failing rather than merging when it cannot.
func (r Repo) MergeFFOnly(ctx context.Context, merge worktree.Merge) error {
	onto, err := r.refName(merge.Onto)
	if err != nil {
		return err
	}

	_, err = r.at(merge.Path).git(ctx, "merge", "--ff-only", onto)
	return err
}
