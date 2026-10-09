package kernel_test

import (
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// The JSON names are the contract agents read: this pins them.
func TestAnOutcomeReadsAsTheJSONAgentsRead(t *testing.T) {
	outcome := kernel.Outcome[string]{
		Items: []kernel.Item[string]{
			{Subject: "feat/a", Status: kernel.StatusDone, Detail: "/wt/a"},
			{Subject: "feat/b", Status: kernel.StatusFailed, Detail: "", Error: &kernel.Error{
				Kind:   kernel.KindInternal,
				Code:   kernel.CodeUndoFailed,
				Params: map[string]string{kernel.ParamStep: "meta", kernel.ParamLeft: "worktree,meta"},
				Cause:  errBoom,
				FollowUp: &kernel.FollowUp{
					Command: "clean", Request: json.RawMessage(`{"branch":"feat/b","force":true}`), Code: "test.left_behind",
				},
			}},
			{Subject: "feat/c", Status: kernel.StatusSkipped, Reason: kernel.ReasonInterrupted},
		},
		Warnings: []kernel.Warning{{Code: "test.diverged", Params: map[string]string{"branch": "main"}}},
	}
	got, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"items":[` +
		`{"subject":"feat/a","status":"done","detail":"/wt/a"},` +
		`{"subject":"feat/b","status":"failed","error":{"kind":"internal","code":"unit.undo_failed","params":{"left":"worktree,meta","step":"meta"},` +
		`"follow_up":{"command":"clean","request":{"branch":"feat/b","force":true},"code":"test.left_behind"}},"detail":""},` +
		`{"subject":"feat/c","status":"skipped","reason":"interrupted","detail":""}],` +
		`"warnings":[{"code":"test.diverged","params":{"branch":"main"}}]}`
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestAFieldErrorReadsAsTheJSONAgentsRead(t *testing.T) {
	problem := kernel.FieldError{Path: "isolation", Code: kernel.CodeOneOf, Params: map[string]string{kernel.ParamValue: "x"}, Accepted: []string{"isolated", "verbatim"}}
	got, err := json.Marshal(kernel.Invalid([]kernel.FieldError{problem}))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind":"invalid","code":"request.invalid","fields":[{"path":"isolation","code":"one_of","params":{"value":"x"},"accepted":["isolated","verbatim"]}]}`
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
