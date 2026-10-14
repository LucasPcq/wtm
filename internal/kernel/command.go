package kernel

import "context"

type Plan[P any] struct {
	Detail   P         `json:"detail"`
	Warnings []Warning `json:"warnings,omitempty"`
}

type ApplyInput[Req, F, P, D any] struct {
	Request    Req
	Facts      F
	Plan       Plan[P]
	Emit       Emitter
	Shield     Shield
	Milestones Milestones[D]
}

type Command[Req, F, P, D any] struct {
	Name    string
	Observe func(context.Context) (F, error)
	Forget  func(F) F
	Fields  []FieldDef[Req, F]
	Rules   Rules[Req, F]
	Locks   func(Req) []LockKey
	Plan    func(context.Context, Req, F) (Plan[P], error)
	Apply   func(context.Context, ApplyInput[Req, F, P, D]) (Outcome[D], error)
}

type Query[Req, V any] struct {
	Name   string
	Run    func(context.Context, Req) (V, error)
	Stream func(context.Context, Req, Emitter) error
}
