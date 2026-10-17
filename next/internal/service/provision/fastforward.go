package provision

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

type FastForwardParams struct {
	Subject      string
	CheckedOutAt string
}

type FastForwarded struct {
	Dirty bool
	Err   error
}

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
