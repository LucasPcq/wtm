package output

import (
	"errors"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

var meta = worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvExample, Isolation: project.Isolated}

const (
	cwd   = "/repo/main"
	trees = "/repo/main/.trees"
)

func twoCreated(trees string) create.Outcome {
	return create.Outcome{
		Op: "op-1",
		Plan: create.Plan{
			Branches: []create.BranchPlan{
				{Branch: "feat/a", Path: filepath.Join(trees, "feat-a"), Action: create.ActionCreate},
				{Branch: "feat/b", Path: filepath.Join(trees, "feat-b"), Action: create.ActionCreate},
			},
			From:    "main",
			EnvFrom: project.EnvExample,
		},
		Fingerprint: "0123456789abcdef01234567",
		Result: &create.Result{Items: []create.Item{
			{Subject: "feat/a", Status: domain.StatusDone, Detail: &create.Detail{Path: filepath.Join(trees, "feat-a"), Metadata: meta}},
			{Subject: "feat/b", Status: domain.StatusDone, Detail: &create.Detail{Path: filepath.Join(trees, "feat-b"), Metadata: meta}},
		}},
	}
}

func mixed(trees string) create.Outcome {
	hook := &domain.Error{Kind: domain.KindInternal, Code: "unit.phase_failed", Message: "on_create failed", Params: domain.Params{"phase": "on_create"}, Cause: errors.New("hook failed: exit status 3")}
	return create.Outcome{
		Op: "op-2",
		Plan: create.Plan{
			Branches: []create.BranchPlan{
				{Branch: "feature", Path: filepath.Join(trees, "feature"), Action: create.ActionCheckOut},
				{Branch: "linked", Path: "/repo/main/elsewhere/linked", Action: create.ActionUnchanged},
				{Branch: "feat/c", Path: filepath.Join(trees, "feat-c"), Action: create.ActionCreate},
				{Branch: "feat/d", Path: filepath.Join(trees, "feat-d"), Action: create.ActionCreate},
				{Branch: "remote-only", Path: filepath.Join(trees, "remote-only"), Action: create.ActionResume, Upstream: "origin/remote-only"},
			},
			From:      "main",
			EnvFrom:   project.EnvMain,
			Isolation: project.Isolated,
		},
		Warnings: []domain.Warning{{Code: "create.ff_dirty", Message: "couldn't fast-forward main: its worktree has uncommitted changes — it starts from your local branch", Params: domain.Params{"branch": "main"}}},
		Result: &create.Result{Items: []create.Item{
			{Subject: "feature", Status: domain.StatusDone, Detail: &create.Detail{
				Path: filepath.Join(trees, "feature"), Metadata: meta,
				Standing: create.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 2},
				EnvPorts: env.Redacted{Offset: 10, Applied: true, Entries: []env.Entry{
					{File: ".env", Key: "PORT", Port: "http", Base: 3000, Resolved: 3010, Status: env.StatusRewrite},
					{File: ".env", Key: "DB", Port: "pg", Base: 5432, Status: env.StatusMissingKey},
				}},
			}},
			{Subject: "linked", Status: domain.StatusDone, Reason: domain.ReasonUnchanged, Detail: &create.Detail{Path: "/repo/main/elsewhere/linked", Metadata: worktree.Meta{SourceBranch: "main"}}},
			{Subject: "feat/c", Status: domain.StatusFailed, Reason: domain.ReasonIncomplete, Err: hook, Warnings: []domain.Warning{{Code: "provision.env_no_template", Message: "no template found for .env, skipping", Params: domain.Params{"target": ".env"}}}, Detail: &create.Detail{Path: filepath.Join(trees, "feat-c"), Metadata: meta}},
			{Subject: "feat/d", Status: domain.StatusCancelled, Reason: domain.ReasonIncomplete, Err: &domain.Error{Kind: domain.KindCancelled, Code: "unit.interrupted", Message: "interrupted", Params: domain.Params{"phase": "on_create"}}, Detail: &create.Detail{Path: filepath.Join(trees, "feat-d"), Metadata: meta}},
			{Subject: "remote-only", Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted},
		}},
	}
}

func invalid() *domain.Error {
	return domain.Invalid([]domain.FieldError{
		{Path: "branches[0]", Code: "create.checked_out_elsewhere", Template: "branch {branch} is checked out at {path} — run `wtm go {branch}` to jump in", Params: domain.Params{"branch": "feat/x", "path": "/p"}},
		{Path: "branches[1]", Code: "distinct", Template: "{path}: {value} is given twice", Params: domain.Params{"value": "feat/x"}},
		{Path: "env_from", Code: "one_of", Template: "{path} must be one of {accepted}, not {value}", Params: domain.Params{"value": "copy"}, Accepted: []string{"example", "main", "parent"}},
		{Path: "from", Code: "required", Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)", Params: domain.Params{"because": "create.parent_required"}},
	})
}
