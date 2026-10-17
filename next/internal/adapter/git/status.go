package git

import (
	"context"
	"strings"
)

// Dirty reports whether the worktree at path has changes, untracked files
// included, as git status shows them.
func (r Repo) Dirty(ctx context.Context, path string) (bool, error) {
	out, err := Repo{Dir: path, Remote: r.Remote}.git(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(out)) != "", nil
}
