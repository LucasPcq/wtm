// Package worktree decides what becomes of a worktree: whether one asked for is added, already there or refused, whether wtm manages it, and what its detached HEAD is.
package worktree

import "github.com/LucasPcq/wtm/internal/domain"

//go-sumtype:decl Creation

// Creation is what a worktree asked for becomes, decided before git is asked
// to add anything.
type Creation interface{ creation() }

type Add struct{}

// Unchanged is --if-not-exists finding the branch already open: Path is where.
type Unchanged struct {
	Path string
}

// CheckedOutElsewhere is a branch git allows out once, already out at Path.
type CheckedOutElsewhere struct {
	Path string
}

// PathTaken is a folder already there. Occupant is the branch of the worktree
// registered at it, empty when none is or its HEAD is detached; another
// branch there is a name clash, two branches sanitizing to one folder.
type PathTaken struct {
	Occupant string
}

func (Add) creation()                 {}
func (Unchanged) creation()           {}
func (CheckedOutElsewhere) creation() {}
func (PathTaken) creation()           {}

type CreationParams struct {
	Branch string
	Path   string
	// CheckedOutAt is empty when no worktree has the branch out.
	CheckedOutAt string
	PathTaken    bool
	Occupant     string
	IfNotExists  bool
}

// CreationOf asks about the branch before the folder: a folder holding another
// branch under the same sanitized name is never an idempotent success.
func CreationOf(params CreationParams) Creation {
	if params.CheckedOutAt != "" {
		if params.IfNotExists {
			return Unchanged{Path: params.CheckedOutAt}
		}
		return CheckedOutElsewhere{Path: params.CheckedOutAt}
	}
	if !params.PathTaken {
		return Add{}
	}
	if params.IfNotExists && params.Occupant == params.Branch {
		return Unchanged{Path: params.Path}
	}
	return PathTaken{Occupant: params.Occupant}
}

// Managed is a worktree wtm created or adopted: one it recorded a creation for.
func Managed(meta domain.WorktreeMetadata) bool {
	return meta.CreatedAt != ""
}
