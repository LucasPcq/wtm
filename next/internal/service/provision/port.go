package provision

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository a worktree is added to and a source fast-forwarded in.
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

// Meta is the store of each worktree's meta.json record.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
	List() ([]string, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
	// LockOrdinals runs do under the lock that serialises ordinal allocation and
	// reports whether the lock was held; do runs unlocked when it could not be.
	LockOrdinals(ctx context.Context, do func() error) (bool, error)
}

// EnvFiles reads, copies and edits a worktree's .env files.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
	Copy(ctx context.Context, request env.CopyRequest) (env.Copied, error)
	Edit(ctx context.Context, edit dotenv.Edit) (domain.Written, error)
}

// AnswerStore keeps the answers a user asked to remember, keyed by their id in config.toml.
type AnswerStore interface {
	Remember(ctx context.Context, answers map[string]string) (domain.Written, error)
}
