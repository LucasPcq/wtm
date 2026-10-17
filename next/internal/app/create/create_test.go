package create_test

import (
	"context"
	"errors"
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
	planned := decided(t, w.facts(t, branchName), create.Request{Branches: []string{branchName}, From: "main", EnvFrom: project.EnvMain})
	require.Equal(t, []create.BranchPlan{{Branch: branchName, Path: w.path(branchName), Action: create.ActionCreate}}, planned.Branches)
	writeFile(t, w.path(branchName), "a file where the worktree goes")

	result := w.deps.Apply(t.Context(), planned, (&recorder{}).add)

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

func TestTheSourceIsFastForwardedBeforeTheWorktreeStartsFromIt(t *testing.T) {
	w := newWorld(t, seed{Behind: true, NoEnv: true})

	result, report := w.apply(t, create.Request{Branches: []string{branchName}, FastForward: new(true)})

	assert.Empty(t, result.Warnings)
	upstream := gittest.Output(t, w.main, "rev-parse", "origin/main")
	assert.Equal(t, upstream, gittest.Output(t, w.main, "rev-parse", "main"))
	assert.Equal(t, upstream, gittest.Output(t, w.path(branchName), "rev-parse", "HEAD"))
	assert.Equal(t, []wtm.Event{
		wtm.PhaseStarted{Subject: "main", Phase: "fast_forward"},
		wtm.PhaseFinished{Subject: "main", Phase: "fast_forward"},
	}, report.all()[:2])
}

func TestASourceWithUncommittedChangesIsLeftWhereItIsAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, FastForward: new(true)})

	assert.Equal(t, []wtm.Warning{{
		Code:    "create.ff_dirty",
		Message: "couldn't fast-forward main: its worktree has uncommitted changes — it starts from your local branch",
		Params:  wtm.Params{"branch": "main"},
	}}, result.Warnings)
	assert.Equal(t, wtm.StatusDone, result.Items[0].Status)
	assert.NotEqual(t, gittest.Output(t, w.main, "rev-parse", "origin/main"), gittest.Output(t, w.path(branchName), "rev-parse", "HEAD"))
}

func TestASourceWithoutWorktreeIsMovedWithoutCheckingItOut(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "push", "-q", "origin", gittest.Output(t, w.main, "commit-tree", existing+"^{tree}", "-p", existing, "-m", "upstream")+":refs/heads/"+existing)
	gittest.Git(t, w.main, "fetch", "-q", "origin")

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, From: existing, FastForward: new(true)})

	assert.Empty(t, result.Warnings)
	assert.Equal(t, gittest.Output(t, w.main, "rev-parse", "origin/"+existing), gittest.Output(t, w.main, "rev-parse", existing))
}

func TestAFastForwardThatFailsIsAWarningAndTheBranchStillStarts(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)
	gittest.Git(t, w.main, "remote", "set-url", "origin", w.root+"/gone")

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, From: existing, FastForward: new(true)})

	assert.Equal(t, []string{"create.ff_failed"}, codes(result.Warnings))
	assert.Contains(t, result.Warnings[0].Message, "couldn't fast-forward feature: ")
	assert.Contains(t, result.Warnings[0].Message, " — it starts from your local branch")
	assert.Equal(t, wtm.StatusDone, result.Items[0].Status)
}

func TestTheAnswersAskedToBeRememberedAreKeptOnceABranchIsCreated(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, EnvFrom: project.EnvExample, Isolation: project.Verbatim, FastForward: new(false), Remember: []string{"env_from", "isolation", "fast_forward"}})

	assert.Empty(t, result.Warnings)
	config := readFile(t, filepath.Join(w.stateDir, "config.toml"))
	assert.Contains(t, config, `env_strategy = "example"`)
	assert.Contains(t, config, `isolation = "verbatim"`)
	assert.Contains(t, config, `source_update = "keep"`)
}

func TestAnIsolationNotAskedIsNotRemembered(t *testing.T) {
	w := newWorld(t, seed{})

	w.apply(t, create.Request{Branches: []string{branchName}, Remember: []string{"isolation", "env_from"}})

	config := readFile(t, filepath.Join(w.stateDir, "config.toml"))
	assert.Contains(t, config, `env_strategy = "main"`)
	assert.NotContains(t, config, "isolation")
}

func TestAFastForwardNotAskedNeverOverwritesTheOneRemembered(t *testing.T) {
	w := newWorld(t, seed{Config: projectToml + "\n[wizard.remembered]\nsource_update = \"ff\"\n"})

	w.apply(t, create.Request{Branches: []string{branchName}, FastForward: new(false), Remember: []string{"fast_forward"}})

	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `source_update = "ff"`)
}

func TestNothingIsRememberedWhenNoBranchWasCreated(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	w.apply(t, create.Request{Branches: []string{"linked"}, IfNotExists: true, EnvFrom: project.EnvExample, Remember: []string{"env_from"}})

	assert.NotContains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), "env_strategy")
}

func TestAnAnswerThatCannotBeRememberedIsAWarningOfTheRun(t *testing.T) {
	w := newWorld(t, seed{})
	w.deps.Remember = func(context.Context, map[string]string) (wtm.Written, error) {
		return wtm.Written{}, errors.New("read-only file system")
	}

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Remember: []string{"env_from"}})

	assert.Equal(t, []string{"create.remember_failed"}, codes(result.Warnings))
	assert.Equal(t, "could not remember the answer: read-only file system", result.Warnings[0].Message)
	assert.Equal(t, wtm.Params{"cause": "read-only file system"}, result.Warnings[0].Params)
}

func TestARememberedAnswerWrittenWithoutItsLockSaysSo(t *testing.T) {
	w := newWorld(t, seed{})
	w.deps.Remember = func(context.Context, map[string]string) (wtm.Written, error) {
		return wtm.Written{Changed: true, Unlocked: true}, nil
	}

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Remember: []string{"env_from"}})

	assert.Equal(t, []string{"locks.unavailable"}, codes(result.Warnings))
}

func TestARunHoldsTheRepositoryAndEachBranchItCreates(t *testing.T) {
	locks := create.Request{Branches: []string{"feat/b", existing, ""}, From: "main"}.Locks()

	assert.Equal(t, []wtm.Lock{
		{Scope: wtm.ScopeRepo, Mode: wtm.LockShared},
		{Scope: wtm.ScopeWorktree, Name: "feat/b", Mode: wtm.LockExclusive},
		{Scope: wtm.ScopeWorktree, Name: existing, Mode: wtm.LockExclusive},
	}, locks)
}

func TestARunThatFastForwardsTheSourceHoldsItToo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	facts := w.facts(t, branchName, "main")
	source := wtm.Lock{Scope: wtm.ScopeWorktree, Name: "main", Mode: wtm.LockExclusive}

	moved := decided(t, facts, create.Request{Branches: []string{branchName}, From: "main", FastForward: new(true)}).Locks()
	kept := decided(t, facts, create.Request{Branches: []string{branchName}, From: "main", FastForward: new(false)}).Locks()
	fromRemote := decided(t, facts, create.Request{Branches: []string{branchName}, From: "origin/main", FastForward: new(true)}).Locks()

	assert.Contains(t, moved, source)
	assert.NotContains(t, kept, source)
	assert.Equal(t, []wtm.Lock{{Scope: wtm.ScopeRepo, Mode: wtm.LockShared}, {Scope: wtm.ScopeWorktree, Name: branchName, Mode: wtm.LockExclusive}}, fromRemote)
}

func TestALocalSourceNamedLikeARemoteOneIsLockedWhenItIsFastForwarded(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "origin/foo", "main")
	ahead := gittest.Output(t, w.main, "commit-tree", "main^{tree}", "-p", "main", "-m", "upstream")
	gittest.Git(t, w.main, "update-ref", "refs/remotes/origin/origin/foo", ahead)
	req := create.Request{Branches: []string{branchName}, From: "origin/foo", FastForward: new(true)}
	facts := w.facts(t, branchName)

	planned, _, err := create.Decide(facts, req)
	require.NoError(t, err)

	assert.Equal(t, []string{"origin/foo"}, planned.FastForward)
	assert.Contains(t, planned.Locks(), wtm.Lock{Scope: wtm.ScopeWorktree, Name: "origin/foo", Mode: wtm.LockExclusive})
}

func TestASourceThatWillNotMoveIsNotLocked(t *testing.T) {
	w := newWorld(t, seed{})
	req := create.Request{Branches: []string{branchName}, From: "main", FastForward: new(true)}

	assert.NotContains(t, decided(t, w.facts(t, branchName), req).Locks(), wtm.Lock{Scope: wtm.ScopeWorktree, Name: "main", Mode: wtm.LockExclusive})
}
