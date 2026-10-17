package events

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// repo, operation, unit and message are the shape of an event on the wire, as
// wtm's daemon and wtm events read it (schema v1).
type repo struct {
	Root      string `json:"root"`
	CommonDir string `json:"common_dir"`
}

type operation struct {
	Command string `json:"command"`
	Surface string `json:"surface"`
}

type unit struct {
	Subject string `json:"subject"`
	Status  string `json:"status"`
}

type message struct {
	V             int                `json:"v"`
	Type          string             `json:"type"`
	TS            string             `json:"ts"`
	CorrelationID string             `json:"correlation_id,omitempty"`
	Repo          *repo              `json:"repo,omitempty"`
	Worktree      *worktree.Identity `json:"worktree,omitempty"`
	Changed       []string           `json:"changed,omitempty"`
	OK            *bool              `json:"ok,omitempty"`
	Hook          string             `json:"hook,omitempty"`
	ExitCode      *int               `json:"exit_code,omitempty"`
	OpID          string             `json:"op_id,omitempty"`
	Operation     *operation         `json:"operation,omitempty"`
	Unit          *unit              `json:"unit,omitempty"`
	Code          string             `json:"code,omitempty"`
}

// messageOf returns the wire message of event and reports whether wtm
// publishes it at all; units started, phases and output stay in the process.
func messageOf(event domain.Event) (message, bool) {
	switch e := event.(type) {
	case domain.WorktreeCreated:
		return message{Type: "worktree.created", Worktree: &e.Worktree}, true
	case domain.WorktreeNumbered:
		return message{Type: "worktree.updated", Worktree: &e.Worktree, Changed: []string{"ordinal"}}, true
	case domain.WorktreeProvisioned:
		return message{Type: "worktree.provisioned", Worktree: &e.Worktree, OK: &e.OK, Hook: e.Hook, ExitCode: e.ExitCode}, true
	case domain.OperationStarted:
		return message{Type: "operation.started", Operation: &operation{Command: e.Command, Surface: e.Surface}}, true
	case domain.UnitFinished:
		return message{Type: "unit.finished", Unit: &unit{Subject: e.Subject, Status: string(e.Status)}}, true
	case domain.OperationFinished:
		return message{Type: "operation.finished", Operation: &operation{Command: e.Command, Surface: e.Surface}, OK: &e.OK, Code: e.Code}, true
	case domain.UnitStarted, domain.PhaseStarted, domain.PhaseFinished, domain.Output:
	}
	return message{}, false
}
