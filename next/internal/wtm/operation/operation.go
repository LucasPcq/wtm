// Package operation is an operation in progress as its holder file records it, and the locks it holds, shared with wtm.
package operation

import "time"

type Operation struct {
	ID            string    `json:"op_id"`
	PID           int       `json:"pid"`
	Command       string    `json:"command"`
	Surface       string    `json:"surface"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	Locks         []Lock    `json:"locks"`
}

type Scope string

const (
	ScopeRepo     Scope = "repo"
	ScopeWorktree Scope = "wt"
)

type Mode string

const (
	Shared    Mode = "shared"
	Exclusive Mode = "exclusive"
)

type Lock struct {
	Scope Scope  `json:"scope"`
	Name  string `json:"name,omitempty"`
	Mode  Mode   `json:"mode"`
}

func (l Lock) Key() string {
	if l.Scope == ScopeWorktree {
		return string(l.Scope) + ":" + l.Name
	}
	return string(l.Scope)
}
