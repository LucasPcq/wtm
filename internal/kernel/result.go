package kernel

import (
	"encoding/json"
	"slices"
)

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
	ReasonIncomplete  Reason = "incomplete"
)

type Item[T any] struct {
	Subject  string    `json:"subject"`
	Status   Status    `json:"status"`
	Reason   Reason    `json:"reason,omitempty"`
	Error    Error     `json:"error,omitempty"`
	Warnings []Warning `json:"warnings,omitempty"`
	Detail   T         `json:"detail"`
}

type Outcome[T any] struct {
	Items     []Item[T]  `json:"items"`
	FollowUps []FollowUp `json:"follow_ups,omitempty"`
	Warnings  []Warning  `json:"warnings,omitempty"`
}

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

type ItemOf[T any] struct {
	subject  string
	warnings []Warning
}

func ItemFor[T any](subject string) ItemOf[T] {
	return ItemOf[T]{subject: subject}
}

func (of ItemOf[T]) Warned(warnings ...Warning) ItemOf[T] {
	of.warnings = append(slices.Clip(of.warnings), warnings...)
	return of
}

func (of ItemOf[T]) Done(detail T) Item[T] {
	return of.item(Item[T]{Status: StatusDone, Detail: detail})
}

func (of ItemOf[T]) Unchanged(detail T) Item[T] {
	return of.item(Item[T]{Status: StatusDone, Reason: ReasonUnchanged, Detail: detail})
}

func (of ItemOf[T]) Skipped(reason Reason) Item[T] {
	return of.item(Item[T]{Status: StatusSkipped, Reason: reason})
}

func (of ItemOf[T]) Failed(err Error, detail T) Item[T] {
	return of.item(Item[T]{Status: StatusFailed, Error: err, Detail: detail})
}

func (of ItemOf[T]) Cancelled(err Error, detail T) Item[T] {
	return of.item(Item[T]{Status: StatusCancelled, Reason: ReasonInterrupted, Error: err, Detail: detail})
}

func (of ItemOf[T]) Incomplete(err Error, detail T) Item[T] {
	status := StatusFailed
	if err.Kind() == KindCancelled {
		status = StatusCancelled
	}
	return of.item(Item[T]{Status: status, Reason: ReasonIncomplete, Error: err, Detail: detail})
}

func (of ItemOf[T]) item(item Item[T]) Item[T] {
	item.Subject = of.subject
	item.Warnings = of.warnings
	return item
}

type RetypeParams[T, U any] struct {
	Item   Item[T]
	Detail U
}

func Retype[T, U any](params RetypeParams[T, U]) Item[U] {
	item := params.Item
	return Item[U]{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Error: item.Error, Warnings: item.Warnings, Detail: params.Detail}
}

func (o Outcome[T]) Failure() Error {
	if at := slices.IndexFunc(o.Items, interrupted[T]); at >= 0 {
		return cancelledError(o.Items[at])
	}
	if at := slices.IndexFunc(o.Items, func(item Item[T]) bool { return item.Status == StatusFailed }); at >= 0 {
		return o.Items[at].Error
	}
	return nil
}

func interrupted[T any](item Item[T]) bool {
	return item.Status == StatusCancelled || item.Reason == ReasonInterrupted
}

func cancelledError[T any](item Item[T]) Error {
	if item.Error != nil {
		return item.Error
	}
	return Cancelled(Problem{Code: CodeInterrupted})
}
