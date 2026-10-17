package provision

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// FastForwardParams names the branch to fast-forward to its origin.
type FastForwardParams struct {
	Subject      string // the local branch
	CheckedOutAt string // the worktree it is checked out in; empty when it is in none
}

// FastForwarded is what FastForward did: Dirty when the branch's worktree had
// changes and was left alone, Err when git failed.
type FastForwarded struct {
	Dirty bool
	Err   error
}

// FastForward brings params.Subject up to its origin branch, never merging:
// the ref alone when no worktree has it checked out, else a fast-forward merge
// in that worktree, which it skips when the worktree is dirty.
func (s Service) FastForward(ctx context.Context, params FastForwardParams) FastForwarded {
	if params.CheckedOutAt == "" {
		return FastForwarded{Err: s.Git.FastForwardRef(ctx, params.Subject)}
	}

	if err := s.Git.FetchRef(ctx, params.Subject); err != nil {
		return FastForwarded{Err: err}
	}

	dirty, err := s.Git.Dirty(ctx, params.CheckedOutAt)
	if err != nil || dirty {
		return FastForwarded{Dirty: dirty, Err: err}
	}

	return FastForwarded{Err: s.Git.MergeFFOnly(ctx, worktree.Merge{Path: params.CheckedOutAt, Onto: worktree.RemoteBranch{Name: params.Subject}})}
}
