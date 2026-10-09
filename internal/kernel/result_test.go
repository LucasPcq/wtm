package kernel_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestEachItemConstructorSetsAConsistentStatus(t *testing.T) {
	failure := kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed})
	interrupted := kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted})
	of := kernel.ItemFor[string]("feat/x")
	cases := []struct {
		name string
		got  kernel.Item[string]
		want kernel.Item[string]
	}{
		{"done", of.Done("/wt/x"), kernel.Item[string]{Subject: "feat/x", Status: kernel.StatusDone, Detail: "/wt/x"}},
		{"unchanged", of.Unchanged("/wt/x"), kernel.Item[string]{Subject: "feat/x", Status: kernel.StatusDone, Reason: kernel.ReasonUnchanged, Detail: "/wt/x"}},
		{"skipped", of.Skipped(kernel.ReasonLocked), kernel.Item[string]{Subject: "feat/x", Status: kernel.StatusSkipped, Reason: kernel.ReasonLocked}},
		{"failed", of.Failed(failure, ""), kernel.Item[string]{Subject: "feat/x", Status: kernel.StatusFailed, Error: failure}},
		{"cancelled", of.Cancelled(interrupted, "/wt/x"), kernel.Item[string]{Subject: "feat/x", Status: kernel.StatusCancelled, Reason: kernel.ReasonInterrupted, Error: interrupted, Detail: "/wt/x"}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.got, c.name)
	}
}

// The JSON names are the contract agents read: these tests pin them.

func TestAnOutcomeReadsAsTheJSONAgentsRead(t *testing.T) {
	left := kernel.Internal(kernel.Problem{
		Code:   kernel.CodeUndoFailed,
		Params: kernel.Params{kernel.ParamStep: "meta", kernel.ParamLeft: "worktree,meta"},
		Cause:  errBoom,
		FollowUp: &kernel.FollowUp{
			Command: "clean", Request: json.RawMessage(`{"branch":"feat/b","force":true}`), Code: "test.left_behind",
		},
	})
	outcome := kernel.Outcome[string]{
		Items: []kernel.Item[string]{
			kernel.ItemFor[string]("feat/a").Done("/wt/a"),
			kernel.ItemFor[string]("feat/b").Failed(left, ""),
			kernel.ItemFor[string]("feat/c").Skipped(kernel.ReasonInterrupted),
		},
		Warnings: []kernel.Warning{{Code: "test.diverged", Params: kernel.Params{"branch": "main"}}},
	}
	got, err := json.Marshal(outcome)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"items": [
			{"subject": "feat/a", "status": "done", "detail": "/wt/a"},
			{"subject": "feat/b", "status": "failed", "detail": "", "error": {
				"kind": "internal", "code": "unit.undo_failed", "params": {"left": "worktree,meta", "step": "meta"},
				"follow_up": {"command": "clean", "request": {"branch": "feat/b", "force": true}, "code": "test.left_behind"}
			}},
			{"subject": "feat/c", "status": "skipped", "reason": "interrupted", "detail": ""}
		],
		"warnings": [{"code": "test.diverged", "params": {"branch": "main"}}]
	}`, string(got))
}

func TestEachKindOfErrorReadsAsTheJSONAgentsRead(t *testing.T) {
	cases := []struct {
		name string
		err  kernel.Error
		want string
	}{
		{
			name: "invalid names its fields",
			err:  kernel.Invalid([]kernel.FieldError{{Path: "isolation", Code: kernel.CodeOneOf, Params: kernel.Params{kernel.ParamValue: "x"}, Accepted: []string{"isolated", "verbatim"}}}),
			want: `{"kind": "invalid", "code": "request.invalid", "fields": [{"path": "isolation", "code": "one_of", "params": {"value": "x"}, "accepted": ["isolated", "verbatim"]}]}`,
		},
		{
			name: "refused names its blockers",
			err:  &kernel.RefusedError{Problem: kernel.Problem{Code: "test.unsafe"}, Blockers: []kernel.Blocker{{Code: "test.dirty", Field: "force"}}},
			want: `{"kind": "refused", "code": "test.unsafe", "blockers": [{"code": "test.dirty", "field": "force"}]}`,
		},
		{
			name: "any other kind carries its problem only",
			err:  kernel.Conflict(kernel.Problem{Code: "lock.held", Params: kernel.Params{"holder": "wtm ui"}, Cause: errBoom}),
			want: `{"kind": "conflict", "code": "lock.held", "params": {"holder": "wtm ui"}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := json.Marshal(c.err)
			require.NoError(t, err)
			assert.JSONEq(t, c.want, string(got))
		})
	}
}
