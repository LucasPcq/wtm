package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

type Git interface {
	Snapshot(ctx context.Context) (worktree.Snapshot, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

type Config interface {
	Load() (project.Config, project.Global, error)
	LoadRun() (project.Run, error)
}

type Answers interface {
	Remember(ctx context.Context, answers map[string]string) (domain.Written, error)
}
