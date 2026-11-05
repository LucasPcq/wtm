package runup

import (
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Meta reads a worktree's meta record by its branch, failing with an error wrapping fs.ErrNotExist when it has none: run up reads the ordinal a start numbered a worktree with.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}
