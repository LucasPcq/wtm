package extract

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
	Changes(ctx context.Context, dir string) ([]worktree.Change, error)
	Conflicts(ctx context.Context, check worktree.ConflictCheck) ([]worktree.Conflict, error)
	Move(ctx context.Context, move worktree.Move) (worktree.Moved, error)
}

type Config interface {
	Load() (project.Config, project.Global, error)
	LoadRun() (project.Run, error)
}

type Answers interface {
	Remember(ctx context.Context, answers map[string]string) (domain.Written, error)
}

type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

type Shield func(ctx context.Context) (context.Context, func())
