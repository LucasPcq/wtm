package exec

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as exec reads it: where a path leads, and whether something is there.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// Meta reads a worktree's meta record by its branch; Load fails with an error wrapping fs.ErrNotExist when the branch has none.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// EnvFiles reads the lines of a worktree's .env file, none when it does not exist.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}

// RunHooks runs a chain of hooks, failing with an *env.HookFailure: exec runs its command as a chain of one.
type RunHooks func(ctx context.Context, chain env.HookChain) error

// Syntax parses a shell line without running it, failing with the shell's own message when the shell refuses it.
type Syntax func(ctx context.Context, line string) error
