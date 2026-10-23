package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAReadIsReportedWithoutOperationPlanNorFingerprint(t *testing.T) {
	read := Read[struct {
		Path string `json:"path"`
	}]{
		Items: []Unit[struct {
			Path string `json:"path"`
		}]{{Subject: "feat/x", Status: StatusDone, Detail: &struct {
			Path string `json:"path"`
		}{"/trees/feat-x"}}},
		Warnings:  []Warning{{Code: "list.prs_unavailable", Message: "GitHub not connected"}},
		FollowUps: []FollowUp{{Command: "go", Answers: []Answered{{Path: "worktree", Values: []string{"feat/x"}}}, Code: "list.go", Message: "Go to the worktree"}},
	}

	data, err := json.Marshal(read.Envelope(EnvelopeParams{Command: "list"}))

	require.NoError(t, err)
	assert.JSONEq(t, `{"v": 2, "command": "list", "ok": true,
		"items": [{"subject": "feat/x", "status": "done", "detail": {"path": "/trees/feat-x"}}],
		"follow_ups": [{"command": "go", "answers": [{"path": "worktree", "values": ["feat/x"]}], "code": "list.go", "message": "Go to the worktree"}],
		"warnings": [{"code": "list.prs_unavailable", "message": "GitHub not connected"}]}`, string(data))
}

func TestAReadEndsOnItsRefusalElseOnTheFirstSubjectItFailedToRead(t *testing.T) {
	unreadable := &Error{Kind: KindInternal, Code: "list.unreadable", Message: "can't read /trees/b"}
	refused := &Error{Kind: KindPrecondition, Code: "list.config_not_found", Message: "no wtm config"}
	read := Read[struct{}]{Items: []Unit[struct{}]{{Subject: "a", Status: StatusDone}, {Subject: "b", Status: StatusFailed, Err: unreadable}}}

	assert.Equal(t, unreadable, read.Envelope(EnvelopeParams{Command: "list"}).Err)
	assert.Equal(t, refused, Read[struct{}]{}.Envelope(EnvelopeParams{Command: "list", Err: refused}).Err)
}

func TestAStalePlanIsFollowedUpByItsReplanInTheEnvelope(t *testing.T) {
	stale := &Error{Kind: KindPrecondition, Code: CodePlanStale, Message: "the plan changed"}
	answers := []Answered{{Path: "branches", Values: []string{"feat/x"}}}

	doc := Outcome[struct{}, struct{}]{}.Envelope(EnvelopeParams{Command: "create", Err: stale, Answers: answers})
	refused := Outcome[struct{}, struct{}]{}.Envelope(EnvelopeParams{Command: "create", Err: &Error{Kind: KindRefused, Code: "create.dirty"}, Answers: answers})

	assert.Equal(t, []FollowUp{{Command: "create", Answers: answers, DryRun: true, Code: CodeReplan, Message: ReplanNote}}, doc.FollowUps)
	assert.Empty(t, refused.FollowUps)
}

func TestASubjectNotReadFailsUnderItsCommandsCodeUnlessTheReadWasInterrupted(t *testing.T) {
	broken := errors.New("broken index")

	failed := Unread[struct{}](UnreadParams{Command: "list", Subject: "feat/a", Path: "/w/a", Err: broken})
	cancelled := Unread[struct{}](UnreadParams{Command: "list", Subject: "feat/a", Path: "/w/a", Err: context.Canceled})

	assert.Equal(t, StatusFailed, failed.Status)
	assert.Equal(t, "list.unreadable", failed.Err.Code)
	assert.Equal(t, "couldn't read /w/a", failed.Err.Message)
	assert.ErrorIs(t, failed.Err, broken)
	assert.Equal(t, Unit[struct{}]{Subject: "feat/a", Status: StatusCancelled, Reason: ReasonInterrupted}, cancelled)
}
