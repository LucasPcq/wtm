package output

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/settle"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

var meta = worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvExample, Isolation: run.Isolated}

func inTrees(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	return filepath.Join(root, ".trees")
}

func twoCreated(trees string) app.CreateOutcome {
	return app.CreateOutcome{
		Op: "op-1",
		Plan: create.Planned{
			Branches: []create.BranchPlan{
				{Branch: "feat/a", Path: filepath.Join(trees, "feat-a"), Action: create.ActionCreate},
				{Branch: "feat/b", Path: filepath.Join(trees, "feat-b"), Action: create.ActionCreate},
			},
			From:    "main",
			EnvFrom: project.EnvExample,
		},
		Fingerprint: "0123456789abcdef01234567",
		Result: &create.Result{Items: []create.Item{
			{Subject: "feat/a", Status: wtm.StatusDone, Detail: &create.Detail{Path: filepath.Join(trees, "feat-a"), Metadata: meta}},
			{Subject: "feat/b", Status: wtm.StatusDone, Detail: &create.Detail{Path: filepath.Join(trees, "feat-b"), Metadata: meta}},
		}},
	}
}

func mixed(trees string) app.CreateOutcome {
	hook := &wtm.Error{Kind: wtm.KindInternal, Code: "unit.phase_failed", Message: "on_create failed", Params: wtm.Params{"phase": "on_create"}, Cause: errors.New("hook failed: exit status 3")}
	return app.CreateOutcome{
		Op: "op-2",
		Plan: create.Planned{
			Branches: []create.BranchPlan{
				{Branch: "feature", Path: filepath.Join(trees, "feature"), Action: create.ActionCheckOut},
				{Branch: "linked", Path: filepath.Join(filepath.Dir(trees), "elsewhere", "linked"), Action: create.ActionUnchanged},
				{Branch: "feat/c", Path: filepath.Join(trees, "feat-c"), Action: create.ActionCreate},
				{Branch: "feat/d", Path: filepath.Join(trees, "feat-d"), Action: create.ActionCreate},
				{Branch: "remote-only", Path: filepath.Join(trees, "remote-only"), Action: create.ActionResume, Upstream: "origin/remote-only"},
			},
			From:      "main",
			EnvFrom:   project.EnvMain,
			Isolation: run.Isolated,
		},
		Warnings: []wtm.Warning{{Code: "create.ff_dirty", Message: "couldn't fast-forward main: its worktree has uncommitted changes — it starts from your local branch", Params: wtm.Params{"branch": "main"}}},
		Result: &create.Result{Items: []create.Item{
			{Subject: "feature", Status: wtm.StatusDone, Detail: &create.Detail{
				Path: filepath.Join(trees, "feature"), Metadata: meta,
				Standing: create.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 2},
				EnvPorts: settle.Redacted{Offset: 10, Applied: true, Entries: []settle.Entry{
					{File: ".env", Key: "PORT", Port: "http", Base: 3000, Resolved: 3010, Status: settle.StatusRewrite},
					{File: ".env", Key: "DB", Port: "pg", Base: 5432, Status: settle.StatusMissingKey},
				}},
			}},
			{Subject: "linked", Status: wtm.StatusDone, Reason: wtm.ReasonUnchanged, Detail: &create.Detail{Path: filepath.Join(filepath.Dir(trees), "elsewhere", "linked"), Metadata: worktree.Meta{SourceBranch: "main"}}},
			{Subject: "feat/c", Status: wtm.StatusFailed, Reason: wtm.ReasonIncomplete, Err: hook, Warnings: []wtm.Warning{{Code: "provision.env_no_template", Message: "no template found for .env, skipping", Params: wtm.Params{"target": ".env"}}}, Detail: &create.Detail{Path: filepath.Join(trees, "feat-c"), Metadata: meta}},
			{Subject: "feat/d", Status: wtm.StatusCancelled, Reason: wtm.ReasonIncomplete, Err: &wtm.Error{Kind: wtm.KindCancelled, Code: "unit.interrupted", Message: "interrupted", Params: wtm.Params{"phase": "on_create"}}, Detail: &create.Detail{Path: filepath.Join(trees, "feat-d"), Metadata: meta}},
			{Subject: "remote-only", Status: wtm.StatusSkipped, Reason: wtm.ReasonInterrupted},
		}},
	}
}

func invalid() *wtm.Error {
	return wtm.Invalid([]wtm.FieldError{
		{Path: "branches[0]", Code: "create.checked_out_elsewhere", Template: "branch {branch} is checked out at {path} — run `wtm go {branch}` to jump in", Params: wtm.Params{"branch": "feat/x", "path": "/p"}},
		{Path: "branches[1]", Code: "distinct", Template: "{path}: {value} is given twice", Params: wtm.Params{"value": "feat/x"}},
		{Path: "env_from", Code: "one_of", Template: "{path} must be one of {accepted}, not {value}", Params: wtm.Params{"value": "copy"}, Accepted: []string{"example", "main", "parent"}},
		{Path: "from", Code: "required", Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)", Params: wtm.Params{"because": "create.parent_required"}},
	})
}
