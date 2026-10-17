package provision

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

type Git interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	AddWorktree(ctx context.Context, addition worktree.Addition) error
	RemoveWorktree(ctx context.Context, path string) error
	DeleteBranch(ctx context.Context, name string) error
	FetchRef(ctx context.Context, name string) error
	FastForwardRef(ctx context.Context, name string) error
	MergeFFOnly(ctx context.Context, merge worktree.Merge) error
	Dirty(ctx context.Context, path string) (bool, error)
}

type Meta interface {
	Load(branch string) (worktree.Meta, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
	LockOrdinals(ctx context.Context, do func() error) (bool, error)
}

type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
	Copy(ctx context.Context, request env.CopyRequest) (env.Copied, error)
	Edit(ctx context.Context, edit dotenv.Edit) (domain.Written, error)
}

type Service struct {
	Git        Git
	Meta       Meta
	EnvFiles   EnvFiles
	RunHooks   func(ctx context.Context, chain env.HookChain) error
	PublicPort func(ctx context.Context, configured int) int
	Shield     func(ctx context.Context) (context.Context, func())
	Now        func() time.Time
}
