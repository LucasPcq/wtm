package runup

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Meta reads a worktree's meta record by its branch, failing with an error wrapping fs.ErrNotExist when it has none: run up reads the ordinal a start numbered a worktree with.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// RunConfig reads run.toml as written and writes it whole: run up remembers keeping the other worktrees' jobs there, as concurrency = "parallel", which both wtm binaries read.
type RunConfig interface {
	SavedRun() (project.SavedRun, error)
	SaveRun(ctx context.Context, rewrite project.RunRewrite) (domain.Written, error)
}
