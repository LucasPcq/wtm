package domain

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
