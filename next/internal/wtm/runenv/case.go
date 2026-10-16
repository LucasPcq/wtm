// Package runenv is the environment wtm gives a worktree: its ports, its compose project, the .env values it writes and what its hooks read.
package runenv

import "github.com/LucasPcq/wtm/next/internal/wtm/run"

//go-sumtype:decl Case

type Case interface{ envCase() }

type Verbatim struct{}

type Linked struct{}

func (Verbatim) envCase() {}
func (Linked) envCase()   {}

func CaseOf(isolation run.Isolation) Case {
	if isolation.Effective() == run.Verbatim {
		return Verbatim{}
	}
	return Linked{}
}
