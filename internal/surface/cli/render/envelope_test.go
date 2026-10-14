package render_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestAnEnvelopeAlwaysHasItsVersionCommandAndLists(t *testing.T) {
	var out bytes.Buffer

	require.NoError(t, render.WriteEnvelope(&out, render.EnvelopeParams{Command: "make", OK: true, Wording: wording}))

	assert.JSONEq(t, `{"v":2,"command":"make","ok":true,"follow_ups":[],"warnings":[]}`, out.String())
}

func TestAnAppliedRunWritesItsItemsWithTheirErrorsInWords(t *testing.T) {
	var out bytes.Buffer
	failure := kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed, Params: kernel.Params{kernel.ParamStep: "worktree"}, Cause: errors.New("exit 128")})

	require.NoError(t, render.WriteEnvelope(&out, render.EnvelopeParams{
		Command: "make",
		Op:      "op-1",
		Items: []kernel.Item[json.RawMessage]{
			{Subject: "feat/a", Status: kernel.StatusDone, Detail: json.RawMessage(`"/wt/feat/a"`)},
			{Subject: "feat/b", Status: kernel.StatusFailed, Error: failure, Detail: json.RawMessage(`""`)},
		},
		Warnings: []kernel.Warning{{Code: kernel.CodeLocksUnavailable}},
		Err:      failure,
		Wording:  wording,
	}))

	assert.JSONEq(t, `{
		"v": 2, "command": "make", "ok": false, "op": "op-1",
		"items": [
			{"subject": "feat/a", "status": "done", "detail": "/wt/feat/a"},
			{"subject": "feat/b", "status": "failed", "detail": "",
			 "error": {"kind": "internal", "code": "unit.step_failed", "message": "worktree failed: exit 128", "params": {"step": "worktree"}}}
		],
		"follow_ups": [],
		"warnings": [{"code": "locks.unavailable", "message": "ran without operation locks"}],
		"error": {"kind": "internal", "code": "unit.step_failed", "message": "worktree failed: exit 128", "params": {"step": "worktree"}}
	}`, out.String())
}

func TestAnInvalidRequestNamesEachFieldByItsPathAndItsFlag(t *testing.T) {
	var out bytes.Buffer
	invalid := kernel.Invalid([]kernel.FieldError{
		{Path: "branches", Code: kernel.CodeRequired},
		{Path: "from", Code: kernel.CodeExclusiveWith, With: []string{"target.base"}},
	})

	require.NoError(t, render.WriteEnvelope(&out, render.EnvelopeParams{Command: "make", Err: invalid, Wording: wording}))

	assert.JSONEq(t, `{
		"v": 2, "command": "make", "ok": false, "follow_ups": [], "warnings": [],
		"error": {"kind": "invalid", "code": "request.invalid", "message": "the request is invalid", "fields": [
			{"path": "branches", "code": "required", "flag": "--branches", "message": "--branches is required"},
			{"path": "from", "code": "exclusive_with", "with": ["target.base"], "flag": "--from", "message": "--from cannot be used with --target.base"}
		]}
	}`, out.String())
}

func TestARefusalNamesEachBlockerAndTheFlagThatLiftsIt(t *testing.T) {
	var out bytes.Buffer
	refused := &kernel.RefusedError{
		Problem:  kernel.Problem{Code: kernel.CodeLockHeld, Params: kernel.Params{kernel.ParamKey: "repo", kernel.ParamCommand: "prune", kernel.ParamPID: "7"}},
		Blockers: []kernel.Blocker{{Code: kernel.CodeLocksUnavailable, Field: "force"}, {Code: kernel.CodeInterrupted}},
	}

	require.NoError(t, render.WriteEnvelope(&out, render.EnvelopeParams{Command: "clean", Err: refused, Wording: wording}))

	assert.JSONEq(t, `{
		"v": 2, "command": "clean", "ok": false, "follow_ups": [], "warnings": [],
		"error": {"kind": "refused", "code": "lock.held", "message": "repo is held by prune (pid 7)",
		          "params": {"key": "repo", "command": "prune", "pid": "7"},
		          "blockers": [
			{"code": "locks.unavailable", "field": "force", "flag": "--force", "message": "ran without operation locks"},
			{"code": "unit.interrupted", "message": "interrupted"}
		]}
	}`, out.String())
}

func TestADryRunWritesItsPlanAndFingerprint(t *testing.T) {
	var out bytes.Buffer

	require.NoError(t, render.WriteEnvelope(&out, render.EnvelopeParams{Command: "make", OK: true, Plan: json.RawMessage(`{"branches":["feat/a"]}`), Fingerprint: "abc", Wording: wording}))

	assert.JSONEq(t, `{"v":2,"command":"make","ok":true,"plan":{"branches":["feat/a"]},"fingerprint":"abc","follow_ups":[],"warnings":[]}`, out.String())
}
