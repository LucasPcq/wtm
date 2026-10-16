package wtm

import "github.com/LucasPcq/wtm/next/internal/wtm/worktree"

//go-sumtype:decl Event

type Event interface{ event() }

type UnitStarted struct{ Subject string }

type PhaseStarted struct{ Subject, Phase, Log string }

type PhaseFinished struct {
	Subject, Phase string
	Failed         bool
}

type Output struct{ Subject, Line string }

type UnitFinished struct {
	Subject string
	Status  Status
}

type WorktreeCreated struct{ Worktree worktree.Identity }

type WorktreeNumbered struct{ Worktree worktree.Identity }

type WorktreeProvisioned struct {
	Worktree worktree.Identity
	OK       bool
	Hook     string
	ExitCode *int
}

func (UnitStarted) event()         {}
func (PhaseStarted) event()        {}
func (PhaseFinished) event()       {}
func (Output) event()              {}
func (UnitFinished) event()        {}
func (WorktreeCreated) event()     {}
func (WorktreeNumbered) event()    {}
func (WorktreeProvisioned) event() {}

type Report func(Event)
