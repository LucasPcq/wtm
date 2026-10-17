package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

type Git interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	Branches(ctx context.Context) (worktree.Branches, error)
	AheadBehind(ctx context.Context, name string) (worktree.AheadBehind, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
	FetchRef(ctx context.Context, name string) error
	FastForwardRef(ctx context.Context, name string) error
	MergeFFOnly(ctx context.Context, merge worktree.Merge) error
	Dirty(ctx context.Context, path string) (bool, error)
}

type Config interface {
	Load() (project.Config, project.Global, error)
	LoadRun() (project.Run, error)
}

type Answers interface {
	Remember(ctx context.Context, answers map[string]string) (domain.Written, error)
}
