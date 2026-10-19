package domain

import (
	"encoding/json"
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

	data, err := json.Marshal(read.Envelope("list", nil))

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

	assert.Equal(t, unreadable, read.Envelope("list", nil).Err)
	assert.Equal(t, refused, Read[struct{}]{}.Envelope("list", refused).Err)
}
