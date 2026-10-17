package wtm

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

type Operation struct {
	ID            string    `json:"op_id"`
	PID           int       `json:"pid"`
	Command       string    `json:"command"`
	Surface       string    `json:"surface"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	Locks         []Lock    `json:"locks"`
}

type LockScope string

const (
	ScopeRepo     LockScope = "repo"
	ScopeWorktree LockScope = "wt"
)

type LockMode string

const (
	LockShared    LockMode = "shared"
	LockExclusive LockMode = "exclusive"
)

type Lock struct {
	Scope LockScope `json:"scope"`
	Name  string    `json:"name,omitempty"`
	Mode  LockMode  `json:"mode"`
}

func (l Lock) Key() string {
	if l.Scope == ScopeWorktree {
		return string(l.Scope) + ":" + l.Name
	}
	return string(l.Scope)
}

const correlationIDMaxBytes = 256

func CorrelationProblem(id string) error {
	if len(id) > correlationIDMaxBytes {
		return errors.New("is longer than 256 bytes")
	}
	if strings.IndexFunc(id, unicode.IsControl) >= 0 {
		return errors.New("contains a control character")
	}
	return nil
}
