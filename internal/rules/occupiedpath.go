package rules

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

type OccupiedPathParams struct {
	Branch string
	Path   string
	// Occupant is the branch of the worktree registered at Path, empty when
	// none is or its HEAD is detached.
	Occupant string
}

// OccupiedPathProblem refuses a worktree whose folder is already there. Two
// branches sanitizing to one folder name is a name clash, not an idempotent
// success: the worktree there is the other branch's.
func OccupiedPathProblem(params OccupiedPathParams) error {
	if params.Occupant == "" || params.Occupant == params.Branch {
		return fmt.Errorf("%w: %s", domain.ErrWorktreePathExists, params.Path)
	}
	return fmt.Errorf("%w: "+domain.WorktreeNameClashFmt, domain.ErrWorktreeNameTaken, params.Branch, params.Occupant, filepath.Base(params.Path))
}
