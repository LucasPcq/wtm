// Package operation is the record of an operation in progress, as dispatch's holder file keeps it.
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

// Lock names the repo, or one worktree by its branch.
type Lock struct {
	Scope string `json:"scope"`
	Name  string `json:"name,omitempty"`
	Mode  string `json:"mode"`
}
