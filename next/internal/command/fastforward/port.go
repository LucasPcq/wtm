package fastforward

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as fast-forward reads it: where a path given leads, origin's branches fetched, and the uncommitted changes a worktree holds.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Fetch(ctx context.Context) error
	Changes(ctx context.Context, dir string) ([]worktree.Change, error)
}

// Shield returns a context the first cancel does not reach, and its release.
type Shield func(ctx context.Context) (context.Context, func())
