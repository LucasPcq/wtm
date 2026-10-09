package kernel

import "context"

// ObserveScope is data only: a command gets its ports when it is built,
// never through kernel.
type ObserveScope struct {
	Dir string
}

type LockScope string

const (
	LockRepo     LockScope = "repo"
	LockWorktree LockScope = "wt"
)

type LockMode string

const (
	LockShared    LockMode = "shared"
	LockExclusive LockMode = "exclusive"
)

// LockKey names an operation lock: the repo, or one worktree by its branch.
type LockKey struct {
	Scope LockScope `json:"scope"`
	Name  string    `json:"name,omitempty"`
	Mode  LockMode  `json:"mode"`
}

type Plan[P any] struct {
	Detail   P         `json:"detail"`
	Warnings []Warning `json:"warnings,omitempty"`
}

// ApplyInput hands Apply what dispatch holds: Emit for progress, Shield for
// the sagas Apply runs through Each.
type ApplyInput[Req, F, P any] struct {
	Request Req
	Facts   F
	Plan    Plan[P]
	Emit    Emitter
	Shield  Shield
}

// Command is a mutation: Req the payload, F the facts Observe reads, P the
// plan, D the detail of each item. Only Observe, Choices and Apply do I/O.
type Command[Req, F, P, D any] struct {
	Name    string
	Observe func(context.Context, ObserveScope) (F, error)
	Fields  []FieldDef[Req, F]
	Rules   Rules[Req, F]
	Locks   func(Req) []LockKey
	Plan    func(context.Context, Req, F) (Plan[P], error)
	Apply   func(context.Context, ApplyInput[Req, F, P]) (Outcome[D], error)
}

// Query is a read: Run returns a view, Stream emits until ctx ends.
type Query[Req, V any] struct {
	Name   string
	Run    func(context.Context, Req) (V, error)
	Stream func(context.Context, Req, Emitter) error
}
