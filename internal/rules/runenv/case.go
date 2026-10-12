// Package runenv decides which environment a worktree's .env files and jobs are given: none, the main checkout's own, or a linked worktree's.
package runenv

import "github.com/LucasPcq/wtm/internal/domain"

//go-sumtype:decl Case

type Case interface{ envCase() }

// Verbatim keeps the .env exactly as it was copied: no identity, no port, no
// namespace.
type Verbatim struct{}

// Main is the main checkout: its compose project never follows its branch, and
// its [[env]] keys are plain keys, a value wtm stamped there going back to the
// template's.
type Main struct{}

// Linked is a worktree wtm numbers: ports offset by its ordinal, names derived
// from its branch.
type Linked struct{}

func (Verbatim) envCase() {}
func (Main) envCase()     {}
func (Linked) envCase()   {}

type CaseParams struct {
	// Isolation is the recorded or chosen one; empty reads as isolated.
	Isolation domain.Isolation
	IsMain    bool
}

// CaseOf asks the isolation first: a verbatim worktree is left alone, whatever
// else it is.
func CaseOf(params CaseParams) Case {
	if params.Isolation == domain.IsolationVerbatim {
		return Verbatim{}
	}
	if params.IsMain {
		return Main{}
	}
	return Linked{}
}
