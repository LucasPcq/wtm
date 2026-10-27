package teardown

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as a removal reads and changes it: whether a worktree holds uncommitted changes, what removing worktrees and deleting branches together would lose, the worktrees still listed, and the removal of a worktree and of its branch.
type Git interface {
	Dirty(ctx context.Context, path string) (bool, error)
	Unpushed(ctx context.Context, losing worktree.Losing) (int, error)
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	RemoveWorktree(ctx context.Context, path string) error
	DeleteBranch(ctx context.Context, name string) error
}

// Meta deletes a worktree's meta record by its branch.
type Meta interface {
	Delete(branch string) error
}

// EnvFiles reads the lines of a worktree's .env file, none when it does not exist.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}
