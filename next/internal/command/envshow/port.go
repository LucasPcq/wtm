package envshow

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as env show reads it: where a path given leads.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// Meta reads a worktree's meta record by its branch; a missing record is an error wrapping fs.ErrNotExist.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// Files reads a .env file, none when it does not exist.
type Files interface {
	Read(path string) ([]dotenv.Line, error)
}

// PublicPort is the port a named address announces, given the proxy's configured one; 0 when no proxy serves names.
type PublicPort func(ctx context.Context, configured int) int
