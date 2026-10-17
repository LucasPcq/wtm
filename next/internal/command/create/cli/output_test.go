package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwoBranchesCreatedAreCountedRowedAndPointedAt(t *testing.T) {

	golden(t, "outcome_two_done", rendered(func(w *bytes.Buffer) { drawOutcome(w, outcomeParams{Outcome: twoCreated(trees), Cwd: cwd}) }))
}

func TestAMixedRunNamesEachAnomalyAndEachUnfinishedBranch(t *testing.T) {

	golden(t, "outcome_mixed", rendered(func(w *bytes.Buffer) {
		drawOutcome(w, outcomeParams{Outcome: mixed(trees), Cwd: cwd, FollowUps: []output.NextStep{{Command: "wtm clean feat/e --force", Note: "branch feat/e is left half-created at /trees/feat-e: remove it with `wtm clean feat/e --force`"}}})
	}))
}

func TestARunOfOnlyOpenBranchesIsUnchanged(t *testing.T) {
	outcome := twoCreated(trees)
	for i := range outcome.Result.Items {
		outcome.Result.Items[i].Reason = "unchanged"
		outcome.Plan.Branches[i].Action = create.ActionUnchanged
	}

	golden(t, "outcome_unchanged", rendered(func(w *bytes.Buffer) { drawOutcome(w, outcomeParams{Outcome: outcome, Cwd: cwd}) }))
}

func TestARunWithNothingToDoSaysSo(t *testing.T) {
	golden(t, "outcome_nothing", rendered(func(w *bytes.Buffer) {
		drawOutcome(w, outcomeParams{Outcome: create.Outcome{Result: &create.Result{}}})
	}))
}

func TestADryRunShowsThePlanItsWarningsItsFingerprintAndHowToApplyIt(t *testing.T) {
	outcome := twoCreated(trees)
	outcome.Result = nil
	outcome.Plan.FastForward = []string{"main"}
	outcome.Plan.Remember = []string{"env_from"}
	outcome.Warnings = []domain.Warning{{Code: "create.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}

	golden(t, "dryrun", rendered(func(w *bytes.Buffer) {
		drawDryRun(w, dryRunParams{Outcome: outcome, Apply: "wtm create feat/a feat/b --env-from example --remember env_from --expect-fingerprint 0123456789abcdef01234567"})
	}))
}

func TestAPlanReadsAsItsRowsThenItsWarnings(t *testing.T) {
	plan := twoCreated(trees).Plan

	text := planTextOf(planTextParams{Plan: plan, Warnings: []domain.Warning{{Code: "create.ff_failed", Message: "couldn't fast-forward main"}}})

	assert.Equal(t, "Branches  feat/a, feat/b\nFrom      main\nEnv       example\n\n! couldn't fast-forward main", text)
}

func TestAnAppliedRunWritesEachItemWithItsDetail(t *testing.T) {
	outcome := twoCreated("/trees")
	outcome.Result.Items[1].Detail.Standing = create.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 2}
	outcome.Result.Items[1].Detail.Ordinal = new(2)
	outcome.Warnings = []domain.Warning{{Code: "locks.unavailable", Message: "ran without operation locks"}}

	assert.JSONEq(t, `{
		"v": 2, "command": "create", "ok": true, "op": "op-1",
		"items": [
			{"subject": "feat/a", "status": "done", "detail": {"path": "/trees/feat-a", "metadata": {"source_branch": "main", "created_at": "2026-10-16T12:00:00Z", "env_strategy": "example", "isolation": "isolated"}, "existing_branch": false}},
			{"subject": "feat/b", "status": "done", "detail": {"path": "/trees/feat-b", "metadata": {"source_branch": "main", "created_at": "2026-10-16T12:00:00Z", "env_strategy": "example", "isolation": "isolated"}, "existing_branch": true, "origin_state": "behind", "origin_behind": 2, "ordinal": 2}}
		],
		"follow_ups": [],
		"warnings": [{"code": "locks.unavailable", "message": "ran without operation locks"}]
	}`, enveloped(t, envelopeParams{Outcome: outcome}))
}

func TestAFailedRunIsNotOkAndNamesItsFirstFailure(t *testing.T) {
	outcome := mixed("/trees")

	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(enveloped(t, envelopeParams{Outcome: outcome})), &doc))

	assert.Equal(t, false, doc["ok"])
	assert.Equal(t, map[string]any{"kind": "cancelled", "code": "unit.interrupted", "message": "interrupted", "params": map[string]any{"phase": "on_create"}}, doc["error"])
	items, ok := doc["items"].([]any)
	require.True(t, ok)
	assert.JSONEq(t, `{"subject": "remote-only", "status": "skipped", "reason": "interrupted", "detail": {}}`, jsonOf(t, items[4]))
	assert.JSONEq(t, `{"subject": "feat/c", "status": "failed", "reason": "incomplete",
		"error": {"kind": "internal", "code": "unit.phase_failed", "message": "on_create failed: hook failed: exit status 3", "params": {"phase": "on_create"}},
		"warnings": [{"code": "provision.env_no_template", "params": {"target": ".env"}, "message": "no template found for .env, skipping"}],
		"detail": {"path": "/trees/feat-c", "metadata": {"source_branch": "main", "created_at": "2026-10-16T12:00:00Z", "env_strategy": "example", "isolation": "isolated"}, "existing_branch": false}}`, jsonOf(t, items[2]))
}

func TestADryRunWritesThePlanAndItsFingerprintInsteadOfItems(t *testing.T) {
	outcome := twoCreated("/trees")
	outcome.Result = nil
	outcome.Warnings = []domain.Warning{{Code: "create.env_parent_fallback", Message: "from main"}}

	assert.JSONEq(t, `{
		"v": 2, "command": "create", "ok": true, "op": "op-1",
		"plan": {"branches": [{"branch": "feat/a", "path": "/trees/feat-a", "action": "create"}, {"branch": "feat/b", "path": "/trees/feat-b", "action": "create"}], "from": "main", "env_from": "example"},
		"fingerprint": "0123456789abcdef01234567",
		"follow_ups": [],
		"warnings": [{"code": "create.env_parent_fallback", "message": "from main"}]
	}`, enveloped(t, envelopeParams{Outcome: outcome}))
}

func TestAnInvalidRequestNamesEachFieldByItsPathAndItsFlag(t *testing.T) {
	assert.JSONEq(t, `{
		"v": 2, "command": "create", "ok": false, "op": "op-3",
		"follow_ups": [], "warnings": [],
		"error": {"kind": "invalid", "code": "request.invalid", "message": "the request is invalid", "fields": [
			{"path": "branches[0]", "code": "create.checked_out_elsewhere", "params": {"branch": "feat/x", "path": "/p"}, "flag": "<branches>[0]", "message": "branch feat/x is checked out at /p — run `+"`wtm go feat/x`"+` to jump in"},
			{"path": "branches[1]", "code": "distinct", "params": {"value": "feat/x"}, "flag": "<branches>[1]", "message": "<branches>[1]: feat/x is given twice"},
			{"path": "env_from", "code": "one_of", "params": {"value": "copy"}, "accepted": ["example", "main", "parent"], "flag": "--env-from", "message": "--env-from must be one of example, main, parent, not copy"},
			{"path": "from", "code": "required", "params": {"because": "create.parent_required"}, "flag": "--from", "message": "--from is required: a branch that already exists locally needs its parent named (it can't be inferred, and `+"`wtm sync`"+` needs it)"}
		]}
	}`, enveloped(t, envelopeParams{Outcome: create.Outcome{Op: "op-3"}, Err: invalid()}))
}

func TestAFailureLeftBehindCarriesItsFollowUp(t *testing.T) {
	stuck := &domain.Error{
		Kind: domain.KindInternal, Code: "unit.undo_failed", Message: "could not remove the worktree, left behind: worktree", Params: domain.Params{"left": "worktree"},
		FollowUp: &domain.FollowUp{Command: "clean", Request: json.RawMessage(`{"branches":["feat/x"],"force":true}`), Code: "provision.left_behind", Message: "branch feat/x is left half-created", Params: domain.Params{"branch": "feat/x", "path": "/p"}},
		Cause:    errors.New("env failed: disk full"),
	}

	assert.JSONEq(t, `{
		"v": 2, "command": "create", "ok": false,
		"follow_ups": [], "warnings": [],
		"error": {"kind": "internal", "code": "unit.undo_failed", "message": "could not remove the worktree, left behind: worktree: env failed: disk full", "params": {"left": "worktree"},
			"follow_up": {"command": "clean", "request": {"branches": ["feat/x"], "force": true}, "code": "provision.left_behind", "params": {"branch": "feat/x", "path": "/p"}}}
	}`, enveloped(t, envelopeParams{Err: stuck}))
}

func TestAnEnvelopeNeverEscapesItsHTMLAndNeverLeavesAPlaceholder(t *testing.T) {
	doc := enveloped(t, envelopeParams{Err: invalid()})

	assert.Contains(t, doc, "<branches>[1]")
	assert.NotContains(t, doc, "{path}")
}

func TestAFieldNamesTheOtherFieldsItIsCheckedWith(t *testing.T) {
	selfParent := domain.Invalid([]domain.FieldError{{Path: "from", Code: "self_parent", Template: "{value} cannot be its own parent: pass another value to {path}", Params: domain.Params{"value": "feat/x"}, With: []string{"branches"}}})

	assert.Contains(t, enveloped(t, envelopeParams{Err: selfParent}), `"with": [
          "branches"
        ],`)
}
