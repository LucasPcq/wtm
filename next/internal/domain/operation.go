package domain

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

// Operation is the record of a mutating command in progress, as its holder file stores it and other processes read it.
type Operation struct {
	ID            string    `json:"op_id"`
	PID           int       `json:"pid"`
	Command       string    `json:"command"`
	Surface       string    `json:"surface"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	Locks         []Lock    `json:"locks"`
}

// LockScope is what an operation lock covers: the whole repository, or one worktree.
type LockScope string

// The scopes of a lock.
const (
	ScopeRepo     LockScope = "repo"
	ScopeWorktree LockScope = "wt"
)

// LockMode says whether other operations may hold the same lock at once.
type LockMode string

// The modes of a lock.
const (
	LockShared    LockMode = "shared"
	LockExclusive LockMode = "exclusive"
)

// Lock is one lock an operation holds; Name is the worktree's for ScopeWorktree, empty for ScopeRepo.
type Lock struct {
	Scope LockScope `json:"scope"`
	Name  string    `json:"name,omitempty"`
	Mode  LockMode  `json:"mode"`
}

// Key returns the identity of the lock's resource, the same for its shared and exclusive modes: `repo`, or `wt:<name>`.
func (l Lock) Key() string {
	if l.Scope == ScopeWorktree {
		return string(l.Scope) + ":" + l.Name
	}

	return string(l.Scope)
}

const correlationIDMaxBytes = 256

// CorrelationProblem returns what makes id unfit as a correlation ID (too long, or holding a control character), or nil when it is fit.
func CorrelationProblem(id string) error {
	if len(id) > correlationIDMaxBytes {
		return errors.New("is longer than 256 bytes")
	}
	if strings.IndexFunc(id, unicode.IsControl) >= 0 {
		return errors.New("contains a control character")
	}

	return nil
}
