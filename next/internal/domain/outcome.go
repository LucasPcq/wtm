package domain

import (
	"cmp"
	"slices"
)

// Status is how the work on one subject ended.
type Status string

// The statuses of a subject's work.
const (
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
	StatusCancelled Status = "cancelled"
)

// Reason qualifies a Status: why a subject was done without a change, skipped, or stopped where it stands.
type Reason string

// The reasons that qualify a status.
const (
	ReasonUnchanged   Reason = "unchanged"   // done: the subject already was as asked
	ReasonIncomplete  Reason = "incomplete"  // failed or cancelled partway, its changes so far kept
	ReasonInterrupted Reason = "interrupted" // cancelled, or skipped because the operation was cancelled first
	ReasonNotReached  Reason = "not_reached" // skipped: the operation stopped before reaching it
)

// An Outcome is what a mutating command's run did: the operation it ran under, the plan P and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply). D is what the command reports of each subject.
type Outcome[P, D any] struct {
	Op          string
	Plan        P
	Fingerprint string
	Warnings    []Warning
	Result      *Result[D]
}

// PlanFingerprint is the fingerprint of the plan the run previewed or applied, the one a preview's apply is held to.
func (o Outcome[P, D]) PlanFingerprint() string {
	return o.Fingerprint
}

// Applied reports whether the run holds the Result of an apply.
func (o Outcome[P, D]) Applied() bool {
	return o.Result != nil
}

// Failure is the error the apply ended on, nil when it succeeded or did not run.
func (o Outcome[P, D]) Failure() *Error {
	if o.Result == nil {
		return nil
	}

	return o.Result.Failure()
}

// Envelope is how the run of command ended for every surface, err being the run's refusal if any: its items once applied, else the plan and its fingerprint unless refused.
func (o Outcome[P, D]) Envelope(command string, err *Error) Envelope {
	doc := Envelope{Command: command, Op: o.Op, Err: err}

	switch {
	case o.Result != nil:
		doc.Err = cmp.Or(doc.Err, o.Failure())
		for _, unit := range o.Result.Items {
			doc.Items = append(doc.Items, unit.item())
		}
		doc.Warnings = o.Warnings
	case doc.Err == nil:
		doc.Plan, doc.Fingerprint, doc.Warnings = &o.Plan, o.Fingerprint, o.Warnings
	}

	return doc
}

// A Result is what an apply did: one Unit per subject of the plan, in its order, the warnings of the whole run and the fields whose answer it remembered for the next runs.
type Result[D any] struct {
	Items      []Unit[D]
	Warnings   []Warning
	Remembered []string
}

// Failure is the error the run ends on: the first interruption, else the first failed subject, else nil.
func (r Result[D]) Failure() *Error {
	if at := slices.IndexFunc(r.Items, Unit[D].interrupted); at >= 0 {
		return cmp.Or(r.Items[at].Err, &Error{Kind: KindCancelled, Code: CodeInterrupted, Message: "interrupted"})
	}

	if at := slices.IndexFunc(r.Items, func(unit Unit[D]) bool { return unit.Status == StatusFailed }); at >= 0 {
		return r.Items[at].Err
	}

	return nil
}

// A Unit is what became of one subject: its status and why, the error it failed on, its warnings, and its Detail once there is something to report of it.
type Unit[D any] struct {
	Subject  string
	Status   Status
	Reason   Reason
	Err      *Error
	Warnings []Warning
	Detail   *D
}

func (u Unit[D]) item() Item {
	return Item{Subject: u.Subject, Status: u.Status, Reason: u.Reason, Err: u.Err, Warnings: u.Warnings, Detail: DetailOf(u.Detail)}
}

func (u Unit[D]) interrupted() bool {
	return u.Status == StatusCancelled || u.Reason == ReasonInterrupted
}
