package kernel

import "encoding/json"

type Status string

const (
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
	StatusCancelled Status = "cancelled"
)

type Reason string

const (
	ReasonInterrupted Reason = "interrupted"
	ReasonNotReached  Reason = "not_reached"
	ReasonUnchanged   Reason = "unchanged"
	ReasonUnsafe      Reason = "unsafe"
	ReasonLocked      Reason = "locked"
)

type Item[T any] struct {
	Subject string `json:"subject"`
	Status  Status `json:"status"`
	Reason  Reason `json:"reason,omitempty"`
	Error   *Error `json:"error,omitempty"`
	Detail  T      `json:"detail"`
}

type Outcome[T any] struct {
	Items     []Item[T]  `json:"items"`
	FollowUps []FollowUp `json:"follow_ups,omitempty"`
	Warnings  []Warning  `json:"warnings,omitempty"`
}

// FollowUp proposes a next command, its request already filled in.
type FollowUp struct {
	Command string            `json:"command"`
	Request json.RawMessage   `json:"request,omitempty"`
	Code    Code              `json:"code"`
	Params  map[string]string `json:"params,omitempty"`
}

type Warning struct {
	Code   Code              `json:"code"`
	Params map[string]string `json:"params,omitempty"`
}
