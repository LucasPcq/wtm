package domain

import "github.com/LucasPcq/wtm/next/internal/domain/worktree"

//go-sumtype:decl Event

// Event is one step of an operation as it happens, reported to the progress view and to the processes listening on the events socket.
type Event interface{ event() }

// UnitStarted reports that the work on one subject (a branch) begins.
type UnitStarted struct{ Subject string }

// PhaseStarted reports that a phase of a subject's work begins; Log is the file its output goes to, if any.
type PhaseStarted struct{ Subject, Phase, Log string }

// PhaseFinished reports that a phase of a subject's work ended, and whether it failed.
type PhaseFinished struct {
	Subject, Phase string
	Failed         bool
}

// Output is one line printed by a phase of a subject's work (a hook's output).
type Output struct{ Subject, Line string }

// UnitFinished reports that the work on one subject ended, with its status.
type UnitFinished struct {
	Subject string
	Status  Status
}

// WorktreeCreated reports that a worktree was added.
type WorktreeCreated struct{ Worktree worktree.Identity }

// WorktreeNumbered reports that a worktree received its number (its ports and env values).
type WorktreeNumbered struct{ Worktree worktree.Identity }

// WorktreeProvisioned reports that a worktree's provisioning ended: OK, or the hook that failed and its exit code.
type WorktreeProvisioned struct {
	Worktree worktree.Identity
	OK       bool
	Hook     string
	ExitCode *int
}

// OperationStarted reports that an operation began, from which command and surface.
type OperationStarted struct{ Command, Surface string }

// OperationFinished reports that an operation ended: OK, or the code of its failure.
type OperationFinished struct {
	Command, Surface string
	OK               bool
	Code             string
}

func (UnitStarted) event()         {}
func (PhaseStarted) event()        {}
func (PhaseFinished) event()       {}
func (Output) event()              {}
func (UnitFinished) event()        {}
func (WorktreeCreated) event()     {}
func (WorktreeNumbered) event()    {}
func (WorktreeProvisioned) event() {}
func (OperationStarted) event()    {}
func (OperationFinished) event()   {}

// Report receives the events of an operation as they happen.
type Report func(Event)
