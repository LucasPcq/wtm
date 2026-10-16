package events

import (
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

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

func messageOf(event wtm.Event) (message, bool) {
	switch e := event.(type) {
	case wtm.WorktreeCreated:
		return message{Type: "worktree.created", Worktree: &e.Worktree}, true
	case wtm.WorktreeNumbered:
		return message{Type: "worktree.updated", Worktree: &e.Worktree, Changed: []string{"ordinal"}}, true
	case wtm.WorktreeProvisioned:
		return message{Type: "worktree.provisioned", Worktree: &e.Worktree, OK: &e.OK, Hook: e.Hook, ExitCode: e.ExitCode}, true
	case wtm.OperationStarted:
		return message{Type: "operation.started", Operation: &operation{Command: e.Command, Surface: e.Surface}}, true
	case wtm.UnitFinished:
		return message{Type: "unit.finished", Unit: &unit{Subject: e.Subject, Status: string(e.Status)}}, true
	case wtm.OperationFinished:
		return message{Type: "operation.finished", Operation: &operation{Command: e.Command, Surface: e.Surface}, OK: &e.OK, Code: e.Code}, true
	case wtm.UnitStarted, wtm.PhaseStarted, wtm.PhaseFinished, wtm.Output:
	}
	return message{}, false
}
