// Package runenv decides which environment a worktree's .env files and jobs are given: none, the main checkout's own, or a linked worktree's, and which of its links apply.
package runenv

import "github.com/LucasPcq/wtm/internal/domain"

//go-sumtype:decl Case

type Case interface{ envCase() }

type Verbatim struct{}

type Main struct{}

type Linked struct{}

func (Verbatim) envCase() {}
func (Main) envCase()     {}
func (Linked) envCase()   {}

type CaseParams struct {
	// Isolation empty reads as isolated.
	Isolation domain.Isolation
	IsMain    bool
}

// CaseOf asks the isolation first: a verbatim worktree is left alone, whatever else it is.
func CaseOf(params CaseParams) Case {
	if params.Isolation == domain.IsolationVerbatim {
		return Verbatim{}
	}
	if params.IsMain {
		return Main{}
	}
	return Linked{}
}
