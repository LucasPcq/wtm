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

// Item is what happened to one subject. Build it through ItemFor, which keeps
// status, reason and error consistent.
type Item[T any] struct {
	Subject string `json:"subject"`
	Status  Status `json:"status"`
	Reason  Reason `json:"reason,omitempty"`
	Error   Error  `json:"error,omitempty"`
	Detail  T      `json:"detail"`
}

type Outcome[T any] struct {
	Items     []Item[T]  `json:"items"`
	FollowUps []FollowUp `json:"follow_ups,omitempty"`
	Warnings  []Warning  `json:"warnings,omitempty"`
}

// FollowUp proposes a next command, its request already filled in.
type FollowUp struct {
	Command string          `json:"command"`
	Request json.RawMessage `json:"request,omitempty"`
	Code    Code            `json:"code"`
	Params  Params          `json:"params,omitempty"`
}

type Warning struct {
	Code   Code   `json:"code"`
	Params Params `json:"params,omitempty"`
}

// ItemOf builds the item of one subject: kernel.ItemFor[Detail]("feat/x").Done(detail).
type ItemOf[T any] struct {
	subject string
}

func ItemFor[T any](subject string) ItemOf[T] {
	return ItemOf[T]{subject: subject}
}

func (of ItemOf[T]) Done(detail T) Item[T] {
	return Item[T]{Subject: of.subject, Status: StatusDone, Detail: detail}
}

// Unchanged is done with nothing to do: already in the state asked for.
func (of ItemOf[T]) Unchanged(detail T) Item[T] {
	return Item[T]{Subject: of.subject, Status: StatusDone, Reason: ReasonUnchanged, Detail: detail}
}

func (of ItemOf[T]) Skipped(reason Reason) Item[T] {
	return Item[T]{Subject: of.subject, Status: StatusSkipped, Reason: reason}
}

func (of ItemOf[T]) Failed(err Error, detail T) Item[T] {
	return Item[T]{Subject: of.subject, Status: StatusFailed, Error: err, Detail: detail}
}

// Cancelled is an item an interruption stopped, keeping what was done.
func (of ItemOf[T]) Cancelled(err Error, detail T) Item[T] {
	return Item[T]{Subject: of.subject, Status: StatusCancelled, Reason: ReasonInterrupted, Error: err, Detail: detail}
}
