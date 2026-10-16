package wtm

type Status string

const (
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
	StatusCancelled Status = "cancelled"
)

type Reason string

const (
	ReasonUnchanged   Reason = "unchanged"
	ReasonIncomplete  Reason = "incomplete"
	ReasonInterrupted Reason = "interrupted"
	ReasonNotReached  Reason = "not_reached"
)
