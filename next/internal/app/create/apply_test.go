package create_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachBranchGetsItsWorktreeEnvAndRecord(t *testing.T) {
	w := newWorld(t, seed{})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName, existing, remote}, From: "main"})

	assert.Equal(t, map[string]wtm.Status{branchName: wtm.StatusDone, existing: wtm.StatusDone, remote: wtm.StatusDone}, statuses(result.Items))
	for _, name := range []string{branchName, existing, remote} {
		assert.Equal(t, name, gittest.Output(t, w.path(name), "branch", "--show-current"))
		assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(name), ".env")))
		recorded, err := w.meta.Load(name)
		require.NoError(t, err)
		assert.Equal(t, "main", recorded.SourceBranch)
	}
	assert.Contains(t, gittest.Output(t, w.main, "config", "branch."+remote+".merge"), remote)
}

func TestEachItemDetailsItsWorktreeAndWhereItsBranchStands(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)

	result, _ := w.apply(t, create.Request{Branches: []string{branchName, existing, remote}, From: "main"})

	meta := worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvMain, Isolation: project.Isolated}
	assert.Equal(t, &create.Detail{Path: w.path(branchName), Metadata: meta}, result.Items[0].Detail)
	assert.Equal(t, &create.Detail{Path: w.path(existing), Metadata: meta, Standing: create.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 1}}, result.Items[1].Detail)
	assert.Equal(t, &create.Detail{Path: w.path(remote), Metadata: meta, Standing: create.Standing{Upstream: "origin/" + remote}}, result.Items[2].Detail)
}

func TestTheDetailIsTheJSONOfTheEnvelope(t *testing.T) {
	ordinal := 2
	detail := create.Detail{Path: "/trees/feat-x", Metadata: worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvMain, Isolation: project.Isolated}, Standing: create.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 1}, Ordinal: &ordinal}

	assert.JSONEq(t, `{"path":"/trees/feat-x","metadata":{"source_branch":"main","created_at":"2026-10-16T12:00:00Z","env_strategy":"main","isolation":"isolated"},"existing_branch":true,"origin_state":"behind","origin_behind":1,"ordinal":2}`, jsonOf(t, detail))
}

func TestOneBranchThatFailsLeavesTheOthersCreated(t *testing.T) {
	w := newWorld(t, seed{Config: withHook(`test "$(git branch --show-current)" != feat/y`)})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName, other}})

	assert.Equal(t, map[string]wtm.Status{branchName: wtm.StatusDone, other: wtm.StatusFailed}, statuses(result.Items))
	assert.Equal(t, wtm.ReasonIncomplete, result.Items[1].Reason)
	assert.Equal(t, "unit.phase_failed", result.Items[1].Err.Code)
	assert.Equal(t, w.path(other), result.Items[1].Detail.Path)
	assert.DirExists(t, w.path(other))
}

func TestABranchThatFailsBeforeItsWorktreeHasNoDetail(t *testing.T) {
	w := newWorld(t, seed{})
	writeFile(t, w.path(branchName), "a file where the worktree goes")
	facts := w.facts(t)
	planned := create.Planned{Branches: []create.BranchPlan{{Branch: branchName, Path: w.path(branchName), Action: create.ActionCreate}}, From: "main", EnvFrom: project.EnvMain}

	result := create.Apply(t.Context(), create.ApplyParams{Deps: w.deps, Facts: facts, Plan: planned, Report: (&recorder{}).add})

	assert.Equal(t, wtm.StatusFailed, result.Items[0].Status)
	assert.Nil(t, result.Items[0].Detail)
}

func TestABranchAlreadyOpenIsUnchangedAndNothingIsReported(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: "linked", Change: func(m *worktree.Meta) error { m.SourceBranch = "main"; return nil }})
	require.NoError(t, err)

	result, report := w.apply(t, create.Request{Branches: []string{"linked"}, IfNotExists: true})

	item := result.Items[0]
	assert.Equal(t, wtm.StatusDone, item.Status)
	assert.Equal(t, wtm.ReasonUnchanged, item.Reason)
	assert.Equal(t, &create.Detail{Path: linkedPath(w.root, "linked"), Metadata: worktree.Meta{SourceBranch: "main"}}, item.Detail)
	assert.Empty(t, report.all())
}

func TestEachBranchIsOneUnitOfTheRun(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})

	_, report := w.apply(t, create.Request{Branches: []string{branchName}})

	events := report.all()
	assert.Equal(t, wtm.UnitStarted{Subject: branchName}, events[0])
	assert.Equal(t, wtm.UnitFinished{Subject: branchName, Status: wtm.StatusDone}, events[len(events)-1])
	_, provisioned := events[len(events)-2].(wtm.WorktreeProvisioned)
	assert.True(t, provisioned)
}

func TestAnInterruptedRunSkipsTheBranchesItNeverReached(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("touch started; sleep 5")})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		<-appears(filepath.Join(w.path(branchName), "started"))
		cancel()
	}()

	result, _ := w.applyIn(ctx, t, create.Request{Branches: []string{branchName, other}})

	assert.Equal(t, wtm.StatusCancelled, result.Items[0].Status)
	assert.Equal(t, wtm.ReasonIncomplete, result.Items[0].Reason)
	assert.Equal(t, create.Item{Subject: other, Status: wtm.StatusSkipped, Reason: wtm.ReasonInterrupted}, result.Items[1])
	assert.NoDirExists(t, w.path(other))
}

func TestTheRecordKeepsTheRunIsolationEvenWhereNothingIsIsolated(t *testing.T) {
	w := newWorld(t, seed{Run: "isolation = \"verbatim\"\n", Config: remembering})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}})

	require.Equal(t, wtm.StatusDone, result.Items[0].Status, result.Items[0].Err)
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Equal(t, project.Verbatim, recorded.Isolation)
}

func TestAnIsolationNothingAppliesToIgnoresTheRememberedOne(t *testing.T) {
	w := newWorld(t, seed{Config: remembering})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}})

	require.Equal(t, wtm.StatusDone, result.Items[0].Status, result.Items[0].Err)
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Equal(t, project.Isolated, recorded.Isolation)
}

func TestTheFailureOfARunIsItsFirstInterruptElseItsFirstFailure(t *testing.T) {
	hook := &wtm.Error{Code: "unit.phase_failed"}
	stopped := &wtm.Error{Code: "unit.interrupted"}
	failed := create.Item{Subject: "a", Status: wtm.StatusFailed, Err: hook}

	assert.Nil(t, create.Result{Items: []create.Item{{Status: wtm.StatusDone}}}.Failure())
	assert.Same(t, hook, create.Result{Items: []create.Item{failed}}.Failure())
	assert.Same(t, stopped, create.Result{Items: []create.Item{failed, {Status: wtm.StatusCancelled, Err: stopped}}}.Failure())
	skipped := create.Result{Items: []create.Item{failed, {Status: wtm.StatusSkipped, Reason: wtm.ReasonInterrupted}}}.Failure()
	assert.Equal(t, &wtm.Error{Kind: wtm.KindCancelled, Code: "unit.interrupted", Message: "interrupted"}, skipped)
}
