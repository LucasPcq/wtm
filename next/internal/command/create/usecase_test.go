package create_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

func TestEachBranchGetsItsWorktreeEnvAndRecord(t *testing.T) {
	w := newWorld(t, seed{})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName, existing, remote}, From: "main"})

	assert.Equal(t, map[string]domain.Status{branchName: domain.StatusDone, existing: domain.StatusDone, remote: domain.StatusDone}, statuses(result.Items))
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
	assert.Equal(t, &create.Detail{Path: w.path(existing), Metadata: meta, Standing: worktree.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 1}}, result.Items[1].Detail)
	assert.Equal(t, &create.Detail{Path: w.path(remote), Metadata: meta, Standing: worktree.Standing{Upstream: "origin/" + remote}}, result.Items[2].Detail)
}

func TestTheDetailIsTheJSONOfTheEnvelope(t *testing.T) {
	detail := create.Detail{Path: "/trees/feat-x", Metadata: worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvMain, Ordinal: 2, Isolation: project.Isolated}, Standing: worktree.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 1}}

	assert.JSONEq(t, `{"path":"/trees/feat-x","metadata":{"source_branch":"main","created_at":"2026-10-16T12:00:00Z","env_strategy":"main","ordinal":2,"isolation":"isolated"},"existing_branch":true,"origin_state":"behind","origin_behind":1}`, jsonOf(t, detail))
}

func TestOneBranchThatFailsLeavesTheOthersCreated(t *testing.T) {
	w := newWorld(t, seed{Config: withHook(`test "$(git branch --show-current)" != feat/y`)})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName, other}})

	assert.Equal(t, map[string]domain.Status{branchName: domain.StatusDone, other: domain.StatusFailed}, statuses(result.Items))
	assert.Equal(t, domain.ReasonIncomplete, result.Items[1].Reason)
	assert.Equal(t, "unit.phase_failed", result.Items[1].Err.Code)
	assert.Equal(t, w.path(other), result.Items[1].Detail.Path)
	assert.DirExists(t, w.path(other))
}

func TestABranchThatFailsBeforeItsWorktreeHasNoDetail(t *testing.T) {
	w := newWorld(t, seed{})
	planned := decided(t, w.facts(t, branchName), create.Request{Branches: []string{branchName}, From: "main", Given: provision.Given{EnvFrom: project.EnvMain}})
	require.Equal(t, []create.BranchPlan{{Branch: branchName, Path: w.path(branchName), Action: create.ActionCreate, Phases: []domain.Phase{domain.PhaseWorktree, domain.PhaseEnv, domain.PhaseMeta}}}, planned.Branches)
	writeFile(t, w.path(branchName), "a file where the worktree goes")

	result := w.usecase.Apply(t.Context(), planned, (&operationtest.Recorder{}).Add)

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Nil(t, result.Items[0].Detail)
}

func TestABranchAlreadyOpenIsUnchangedAndNothingIsReported(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: "linked", Change: func(m *worktree.Meta) error { m.SourceBranch = "main"; return nil }})
	require.NoError(t, err)

	result, report := w.apply(t, create.Request{Branches: []string{"linked"}, IfNotExists: true})

	item := result.Items[0]
	assert.Equal(t, domain.StatusDone, item.Status)
	assert.Equal(t, domain.ReasonUnchanged, item.Reason)
	assert.Equal(t, &create.Detail{Path: linkedPath(w.root, "linked"), Metadata: worktree.Meta{SourceBranch: "main"}}, item.Detail)
	assert.Empty(t, report.All())
}

func TestEachBranchIsOneUnitOfTheRun(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})

	_, report := w.apply(t, create.Request{Branches: []string{branchName}})

	events := report.All()
	assert.Equal(t, domain.UnitStarted{Subject: branchName}, events[0])
	assert.Equal(t, domain.UnitFinished{Subject: branchName, Status: domain.StatusDone}, events[len(events)-1])
	_, provisioned := events[len(events)-2].(domain.WorktreeProvisioned)
	assert.True(t, provisioned)
}

func TestNamedAddressesNoProxyServesAreAWarningOfTheWorktree(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}})

	require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	assert.Equal(t, []string{"provision.ports_not_named"}, codes(result.Items[0].Warnings))
	require.NotNil(t, result.Items[0].Warnings[0].FollowUp)
	assert.Equal(t, proxyStatusLine().Command, result.Items[0].Warnings[0].FollowUp.Command)
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

	assert.Equal(t, domain.StatusCancelled, result.Items[0].Status)
	assert.Equal(t, domain.ReasonIncomplete, result.Items[0].Reason)
	assert.Equal(t, create.Item{Subject: other, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}, result.Items[1])
	assert.NoDirExists(t, w.path(other))
}

func TestTheRecordKeepsTheRunIsolationEvenWhereNothingIsIsolated(t *testing.T) {
	w := newWorld(t, seed{Run: "isolation = \"verbatim\"\n", Config: remembering})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}})

	require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Equal(t, project.Verbatim, recorded.Isolation)
}

func TestAnIsolationNothingAppliesToIgnoresTheRememberedOne(t *testing.T) {
	w := newWorld(t, seed{Config: remembering})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}})

	require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Equal(t, project.Isolated, recorded.Isolation)
}

func TestTheFailureOfARunIsItsFirstInterruptElseItsFirstFailure(t *testing.T) {
	hook := &domain.Error{Code: "unit.phase_failed"}
	stopped := &domain.Error{Code: "unit.interrupted"}
	failed := create.Item{Subject: "a", Status: domain.StatusFailed, Err: hook}

	assert.Nil(t, create.Result{Items: []create.Item{{Status: domain.StatusDone}}}.Failure())
	assert.Same(t, hook, create.Result{Items: []create.Item{failed}}.Failure())
	assert.Same(t, stopped, create.Result{Items: []create.Item{failed, {Status: domain.StatusCancelled, Err: stopped}}}.Failure())
	skipped := create.Result{Items: []create.Item{failed, {Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}}}.Failure()
	assert.Equal(t, &domain.Error{Kind: domain.KindCancelled, Code: "unit.interrupted", Message: "interrupted"}, skipped)
}

func TestTheSourceIsFastForwardedBeforeTheWorktreeStartsFromIt(t *testing.T) {
	w := newWorld(t, seed{Behind: true, NoEnv: true})

	result, report := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{FastForward: new(true)}})

	assert.Empty(t, result.Warnings)
	upstream := gittest.Output(t, w.main, "rev-parse", "origin/main")
	assert.Equal(t, upstream, gittest.Output(t, w.main, "rev-parse", "main"))
	assert.Equal(t, upstream, gittest.Output(t, w.path(branchName), "rev-parse", "HEAD"))
	assert.Equal(t, []domain.Event{
		domain.PhaseStarted{Subject: "main", Phase: "fast_forward"},
		domain.PhaseFinished{Subject: "main", Phase: "fast_forward"},
	}, report.All()[:2])
}

func TestABranchFastForwardedStandsAsOriginHasIt(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)

	result, _ := w.apply(t, create.Request{Branches: []string{existing}, From: "main", Given: provision.Given{FastForward: new(true)}})

	require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	assert.Equal(t, worktree.Standing{ExistingBranch: true, OriginState: worktree.UpToDate}, result.Items[0].Detail.Standing)
}

func TestASourceWithUncommittedChangesIsLeftWhereItIsAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []domain.Warning{{
		Code:    "create.ff_dirty",
		Message: "couldn't fast-forward main: its worktree has uncommitted changes — it starts from your local branch",
		Params:  domain.Params{"branch": "main"},
	}}, result.Warnings)
	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.NotEqual(t, gittest.Output(t, w.main, "rev-parse", "origin/main"), gittest.Output(t, w.path(branchName), "rev-parse", "HEAD"))
}

func TestAFastForwardThatFailsIsAWarningAndTheBranchStillStarts(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)
	gittest.Git(t, w.main, "remote", "set-url", "origin", w.root+"/gone")

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, From: existing, Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []string{"create.ff_failed"}, codes(result.Warnings))
	assert.Contains(t, result.Warnings[0].Message, "couldn't fast-forward feature: ")
	assert.Contains(t, result.Warnings[0].Message, " — it starts from your local branch")
	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
}

func TestTheAnswersAskedToBeRememberedAreKeptOnceABranchIsCreated(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{EnvFrom: project.EnvExample, Isolation: project.Verbatim, FastForward: new(false), Remember: []string{"env_from", "isolation", "fast_forward"}}})

	assert.Empty(t, result.Warnings)
	assert.Equal(t, []string{"env_from", "isolation", "fast_forward"}, result.Remembered)
	config := readFile(t, filepath.Join(w.stateDir, "config.toml"))
	assert.Contains(t, config, `env_strategy = "example"`)
	assert.Contains(t, config, `isolation = "verbatim"`)
	assert.Contains(t, config, `source_update = "keep"`)
}

func TestAnIsolationNotAskedIsNotRemembered(t *testing.T) {
	w := newWorld(t, seed{})

	w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{Remember: []string{"isolation", "env_from"}}})

	config := readFile(t, filepath.Join(w.stateDir, "config.toml"))
	assert.Contains(t, config, `env_strategy = "config"`)
	assert.NotContains(t, config, "isolation")
}

func TestAFastForwardNotAskedNeverOverwritesTheOneRemembered(t *testing.T) {
	w := newWorld(t, seed{Config: projectToml + "\n[wizard.remembered]\nsource_update = \"ff\"\n"})

	w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{FastForward: new(false), Remember: []string{"fast_forward"}}})

	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `source_update = "ff"`)
}

func TestNothingIsRememberedWhenNoBranchWasCreated(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	result, _ := w.apply(t, create.Request{Branches: []string{"linked"}, IfNotExists: true, Given: provision.Given{EnvFrom: project.EnvExample, Remember: []string{"env_from"}}})

	assert.Empty(t, result.Remembered)
	assert.NotContains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), "env_strategy")
}

func TestAnAnswerThatCannotBeRememberedIsAWarningOfTheRun(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Provision.Answers = answering{err: errors.New("read-only file system")}

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{Remember: []string{"env_from"}}})

	assert.Equal(t, []string{"create.remember_failed"}, codes(result.Warnings))
	assert.Empty(t, result.Remembered)
	assert.Equal(t, "could not remember the answer: read-only file system", result.Warnings[0].Message)
	assert.Equal(t, domain.Params{"cause": "read-only file system"}, result.Warnings[0].Params)
}

func TestARememberedAnswerWrittenWithoutItsLockSaysSo(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Provision.Answers = answering{written: domain.Written{Changed: true, Unlocked: true}}

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{Remember: []string{"env_from"}}})

	assert.Equal(t, []string{"locks.unavailable"}, codes(result.Warnings))
}

func TestARunHoldsTheRepositoryAndEachBranchItCreates(t *testing.T) {
	locks := create.Request{Branches: []string{"feat/b", existing, ""}, From: "main"}.Locks()

	assert.Equal(t, []domain.Lock{
		{Scope: domain.ScopeRepo, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: "feat/b", Mode: domain.LockExclusive},
		{Scope: domain.ScopeWorktree, Name: existing, Mode: domain.LockExclusive},
	}, locks)
}

func TestARunThatFastForwardsTheSourceHoldsItToo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	facts := w.facts(t, branchName, "main")
	source := domain.Lock{Scope: domain.ScopeWorktree, Name: "main", Mode: domain.LockExclusive}

	moved := decided(t, facts, create.Request{Branches: []string{branchName}, From: "main", Given: provision.Given{FastForward: new(true)}}).Locks()
	kept := decided(t, facts, create.Request{Branches: []string{branchName}, From: "main", Given: provision.Given{FastForward: new(false)}}).Locks()
	fromRemote := decided(t, facts, create.Request{Branches: []string{branchName}, From: "origin/main", Given: provision.Given{FastForward: new(true)}}).Locks()

	assert.Contains(t, moved, source)
	assert.NotContains(t, kept, source)
	assert.Equal(t, []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockShared}, {Scope: domain.ScopeWorktree, Name: branchName, Mode: domain.LockExclusive}}, fromRemote)
}

func TestALocalSourceNamedLikeARemoteOneIsLockedWhenItIsFastForwarded(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "origin/foo", "main")
	ahead := gittest.Output(t, w.main, "commit-tree", "main^{tree}", "-p", "main", "-m", "upstream")
	gittest.Git(t, w.main, "update-ref", "refs/remotes/origin/origin/foo", ahead)
	req := create.Request{Branches: []string{branchName}, From: "origin/foo", Given: provision.Given{FastForward: new(true)}}
	facts := w.facts(t, branchName)

	planned, _ := create.Decide(facts, req)

	assert.Equal(t, []string{"origin/foo"}, planned.FastForward)
	assert.Contains(t, planned.Locks(), domain.Lock{Scope: domain.ScopeWorktree, Name: "origin/foo", Mode: domain.LockExclusive})
}

func TestASourceThatWillNotMoveIsNotLocked(t *testing.T) {
	w := newWorld(t, seed{})
	req := create.Request{Branches: []string{branchName}, From: "main", Given: provision.Given{FastForward: new(true)}}

	assert.NotContains(t, decided(t, w.facts(t, branchName), req).Locks(), domain.Lock{Scope: domain.ScopeWorktree, Name: "main", Mode: domain.LockExclusive})
}

func TestADryRunPlansDespiteAHeldLockAndWritesNothing(t *testing.T) {
	w := newWorld(t, seed{})
	held, err := oplock.Locks{StateDir: w.stateDir}.Take(domain.Operation{ID: "other", Locks: []domain.Lock{{Scope: domain.ScopeWorktree, Name: branchName, Mode: domain.LockExclusive}}})
	require.NoError(t, err)
	defer held.Release()

	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{DryRun: true})

	require.NoError(t, err)
	assert.Equal(t, "op-1", outcome.Op)
	assert.Equal(t, []create.BranchPlan{{Branch: branchName, Path: w.path(branchName), Action: create.ActionCreate, Phases: []domain.Phase{domain.PhaseWorktree, domain.PhaseEnv, domain.PhaseMeta}}}, outcome.Plan.Branches)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{24}$`), outcome.Fingerprint)
	assert.Nil(t, outcome.Result)
	assert.NoDirExists(t, w.path(branchName))
	assert.NoDirExists(t, filepath.Join(w.stateDir, "worktrees"))
}

func TestAPlanCountsTheWorktreesItOpens(t *testing.T) {
	w := newWorld(t, seed{})

	one, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	two, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName, "feat/other"}}, domain.RunOptions{DryRun: true})
	require.NoError(t, err)

	assert.Equal(t, 1, one.Plan.Creates)
	assert.Equal(t, 2, two.Plan.Creates)
}

func TestAWorktreeIsCreatedUnderItsLocksAndAnnouncedFromStartToFinish(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})

	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{Surface: "tui"})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)
	assert.Equal(t, "2026-10-16T12:00:00Z", outcome.Result.Items[0].Detail.Metadata.CreatedAt)
	events := w.published.All()
	assert.Equal(t, domain.OperationStarted{Command: "create", Surface: "tui"}, events[0])
	assert.Equal(t, domain.OperationFinished{Command: "create", Surface: "tui", OK: true}, events[len(events)-1])
	assert.Contains(t, events, domain.Event(domain.UnitFinished{Subject: branchName, Status: domain.StatusDone}))
	assert.True(t, containsKind[domain.WorktreeCreated](events))
	assert.True(t, containsKind[domain.WorktreeUpdated](events))
	assert.True(t, containsKind[domain.WorktreeProvisioned](events))
	assert.NoFileExists(t, filepath.Join(w.stateDir, "locks", "ops", "op-1.json"))
}

func TestARunWhoseBranchFailsFinishesNotOkWithItsCode(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("exit 3")})

	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{Surface: "cli", Report: (&operationtest.Recorder{}).Add})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, outcome.Result.Items[0].Status)
	events := w.published.All()
	assert.Equal(t, domain.OperationFinished{Command: "create", Surface: "cli", Code: "unit.phase_failed"}, events[len(events)-1])
}

func TestARepositoryHeldByAnotherProcessRefusesTheRunAndChangesNothing(t *testing.T) {
	w := newWorld(t, seed{})
	child := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	child.Env = append(os.Environ(), envHelperState+"="+w.stateDir)
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = stdin.Close(); _ = child.Process.Kill(); _ = child.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, readyLine+"\n", line)

	_, refused := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})

	failure := failureOf(t, refused)
	assert.Equal(t, domain.KindConflict, failure.Kind)
	assert.Equal(t, "lock.held", failure.Code)
	assert.Equal(t, "repo", failure.Params["key"])
	assert.Equal(t, strconv.Itoa(child.Process.Pid), failure.Params["pid"])
	assert.NoDirExists(t, w.path(branchName))
	assert.Empty(t, w.published.All())

	require.NoError(t, child.Process.Kill())
	_ = child.Wait()
	_, err = w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})
	assert.NoError(t, err, "a dead process still held the repository")
}

func TestHelperProcess(t *testing.T) {
	stateDir := os.Getenv(envHelperState)
	if stateDir == "" {
		t.Skip("run by TestARepositoryHeldByAnotherProcessRefusesTheRunAndChangesNothing")
	}
	held, err := oplock.Locks{StateDir: stateDir}.Take(domain.Operation{ID: "child", PID: os.Getpid(), Command: "create", Locks: []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockExclusive}}})
	require.NoError(t, err)
	defer held.Release()
	os.Stdout.WriteString(readyLine + "\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestAPreviewStillHoldsWhenAnUnrelatedBranchMoved(t *testing.T) {
	w := newWorld(t, seed{})
	a := w.usecase
	preview, err := a.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{DryRun: true, Report: (&operationtest.Recorder{}).Add})
	require.NoError(t, err)
	gittest.CreateBranch(t, w.main, "unrelated")

	outcome, err := a.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: (&operationtest.Recorder{}).Add})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
}

func TestALockTheDefaultsNameIsTakenThenTheFactsObservedAgain(t *testing.T) {
	w := newWorld(t, seed{Behind: true, Config: projectToml + "\n[wizard.remembered]\nsource_update = \"ff\"\n"})
	observed := 0
	w.usecase.Repository.Git = counted{Repo: w.repo, observed: &observed}
	held, err := oplock.Locks{StateDir: w.stateDir}.Take(domain.Operation{ID: "other", Command: "sync", PID: 7, Locks: []domain.Lock{{Scope: domain.ScopeWorktree, Name: "main", Mode: domain.LockExclusive}}})
	require.NoError(t, err)

	_, refused := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})
	held.Release()
	observed = 0
	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})

	assert.Equal(t, "wt:main", failureOf(t, refused).Params["key"])
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, outcome.Plan.FastForward)
	assert.Equal(t, 2, observed)
}

func TestARunWithoutItsLocksSaysSoOnce(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Operations.Locker = unprotected{}

	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})

	require.NoError(t, err)
	assert.Equal(t, []domain.Warning{{Code: "locks.unavailable", Message: "ran without operation locks"}}, outcome.Warnings)
}

func TestTheWarningsOfThePlanComeWithThoseOfTheRun(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}, From: "feature", Given: provision.Given{EnvFrom: project.EnvParent}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})

	require.NoError(t, err)
	assert.Equal(t, []string{"create.env_parent_fallback"}, codes(outcome.Warnings))
}

func TestTheQuestionsAreAskedOfTheBranchesOfTheRequest(t *testing.T) {
	w := newWorld(t, seed{})
	writeFile(t, filepath.Join(w.path(branchName), "work"), "x")

	questions, err := w.usecase.Questions(t.Context(), create.Request{Branches: []string{branchName}})

	require.NoError(t, err)
	assert.Equal(t, "create.path_exists", questions.Errors[0].Code)
	assert.Equal(t, domain.Answer[string]{Value: "main", Origin: domain.OriginConfig}, questions.From.Answer)
}

func TestAnInvalidRequestIsRefusedBeforeAnythingIsLockedOrWritten(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{"bad..name"}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})

	assert.Equal(t, domain.KindInvalid, failureOf(t, err).Kind)
	assert.NoFileExists(t, filepath.Join(w.stateDir, "locks", "ops", "op-1.json"))
	assert.Empty(t, w.published.All())
}

func TestARunWithoutLocksThatRemembersSaysSoOnce(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Operations.Locker = unprotected{}
	w.usecase.Provision.Answers = answering{written: domain.Written{Changed: true, Unlocked: true}}

	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}, Given: provision.Given{Remember: []string{"env_from"}}}, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})

	require.NoError(t, err)
	assert.Equal(t, []string{"locks.unavailable"}, codes(outcome.Warnings))
}

func TestARunWithoutReportStillRuns(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
}

func TestAnOutcomeFailsOnItsFirstFailedItemAndAPreviewNever(t *testing.T) {
	failure := &domain.Error{Kind: domain.KindInternal, Code: "unit.phase_failed"}
	applied := create.Outcome{Result: &create.Result{Items: []create.Item{{Subject: branchName, Status: domain.StatusFailed, Err: failure}}}}

	assert.Same(t, failure, applied.Failure())
	assert.Nil(t, create.Outcome{}.Failure())
}

func TestAPreviewIsStaleOnceTheIsolationItWouldRecordChanged(t *testing.T) {
	w := newWorld(t, seed{Run: "isolation = \"verbatim\"\n"})
	a := w.usecase
	preview, err := a.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	require.Empty(t, preview.Plan.Isolation)

	writeFile(t, filepath.Join(w.stateDir, "run.toml"), "isolation = \"isolated\"\n")
	_, staleErr := a.Run(t.Context(), create.Request{Branches: []string{branchName}}, domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: (&operationtest.Recorder{}).Add})

	assert.Equal(t, domain.CodePlanStale, failureOf(t, staleErr).Code)
	assert.NoDirExists(t, w.path(branchName))
}

func TestNothingIsRememberedWhenTheOnlyBranchFailsToProvision(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Config: withHook("exit 1")})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation"}}})

	require.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.NotContains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), "isolation")
}

func TestAChangeToWhatTheApplyRunsMakesThePreviewStale(t *testing.T) {
	for name, change := range map[string]func(w world){
		"hooks": func(w world) { writeFile(t, filepath.Join(w.stateDir, "config.toml"), withHook("touch hooked")) },
		"run": func(w world) {
			writeFile(t, filepath.Join(w.stateDir, "run.toml"), strings.Replace(jobsToml, "3000", "4000", 1))
		},
		"env": func(w world) {
			writeFile(t, filepath.Join(w.stateDir, "config.toml"), projectToml+"\n[[env.file]]\ntarget = \"web/.env\"\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, seed{Run: jobsToml})
			req := create.Request{Branches: []string{branchName}}
			preview, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true, Report: (&operationtest.Recorder{}).Add})
			require.NoError(t, err)
			change(w)

			_, err = w.usecase.Run(t.Context(), req, domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: (&operationtest.Recorder{}).Add})

			assert.Equal(t, domain.CodePlanStale, failureOf(t, err).Code)
			assert.NoDirExists(t, w.path(branchName))
		})
	}
}

func TestACancelDuringTheFastForwardIsNoWarningAndSkipsEveryBranch(t *testing.T) {
	w := newWorld(t, seed{Behind: true, NoEnv: true})
	planned, _ := w.plan(t, create.Request{Branches: []string{branchName, other}, Given: provision.Given{FastForward: new(true)}})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	result := w.usecase.Apply(ctx, planned, func(event domain.Event) {
		if started, ok := event.(domain.PhaseStarted); ok && started.Phase == "fast_forward" {
			cancel()
		}
	})

	assert.Empty(t, result.Warnings)
	assert.Equal(t, []create.Item{
		{Subject: branchName, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted},
		{Subject: other, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted},
	}, result.Items)
	assert.Equal(t, domain.CodeInterrupted, result.Failure().Code)
	assert.NoDirExists(t, w.path(branchName))
}

func TestASourceInTheMiddleOfARebaseIsNotFastForwardedAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)
	rebasing(t, w, existing)

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, From: existing, Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []domain.Warning{{
		Code:    "create.ff_busy",
		Message: "couldn't fast-forward feature: its worktree is in the middle of an operation — it starts from your local branch",
		Params:  domain.Params{"branch": existing},
	}}, result.Warnings)
	assert.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
}

func TestTheConfigDefaultFollowsEnvStrategyOverARememberedAnswerAndIsRememberedAsSuch(t *testing.T) {
	w := newWorld(t, seed{Config: projectToml + "\n[wizard.remembered]\nenv_strategy = \"example\"\n"})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{EnvFrom: provision.EnvFromConfig, Remember: []string{"env_from"}}})

	require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
	assert.Equal(t, project.EnvMain, result.Items[0].Detail.Metadata.EnvStrategy)
	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `env_strategy = "config"`)
}

func TestAPreviewIsStaleOnlyWhenTheConfiguredStrategyChangesItsPlan(t *testing.T) {
	cases := []struct {
		name  string
		given provision.Given
		stale bool
	}{
		{name: "explicit", given: provision.Given{EnvFrom: project.EnvMain}},
		{name: "configured", stale: true},
		{name: "followsConfig", given: provision.Given{EnvFrom: provision.EnvFromConfig}, stale: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, seed{})
			req := create.Request{Branches: []string{branchName}, Given: tc.given}
			preview, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true, Report: (&operationtest.Recorder{}).Add})
			require.NoError(t, err)
			writeFile(t, filepath.Join(w.stateDir, "config.toml"), strings.Replace(projectToml, `strategy = "main"`, `strategy = "parent"`, 1))

			_, err = w.usecase.Run(t.Context(), req, domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: (&operationtest.Recorder{}).Add})

			if !tc.stale {
				require.NoError(t, err)
				return
			}
			assert.Equal(t, domain.CodePlanStale, failureOf(t, err).Code)
		})
	}
}

func TestABranchCheckedOutBehindOriginIsWarnedOnItsItem(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)

	result, _ := w.apply(t, create.Request{Branches: []string{branchName, existing}, From: "main"})

	assert.Empty(t, result.Items[0].Warnings)
	assert.Equal(t, []domain.Warning{{Code: "create.behind_origin", Message: "1 commit(s) behind origin — the worktree starts from your local branch.", Params: domain.Params{"behind": "1"}}}, result.Items[1].Warnings)
}

func TestThePlanRecapsWhatItDecidedAndTheAnswersItRecalled(t *testing.T) {
	w := newWorld(t, seed{Config: projectToml + "\n[wizard.remembered]\nenv_strategy = \"example\"\n"})

	planned, _ := w.plan(t, create.Request{Branches: []string{existing}, From: "main"})

	assert.Equal(t, []string{domain.PathEnvFrom}, planned.Recalled)
	assert.Equal(t, []domain.RecapRow{
		{Key: "branches", Label: "Branches", Path: "branches", Values: []domain.RecapValue{{Value: existing, Note: worktree.NoteExisting, Text: existing + " (existing)"}}},
		{Key: "parent", Label: "Parent", Path: "from", Values: domain.RecapValues([]string{"main"})},
		{Key: "env_from", Label: "Env", Path: "env_from", Mark: domain.MarkRecalled, Values: domain.RecapValues([]string{"example"})},
	}, planned.Recap)
}

func TestTheRecapAndTheRecalledAnswersLeaveTheFingerprintAlone(t *testing.T) {
	w := newWorld(t, seed{})
	planned, _ := w.plan(t, create.Request{Branches: []string{branchName}})
	bare := planned
	bare.Recalled, bare.Recap = nil, nil

	assert.NotEmpty(t, planned.Recap)
	assert.Equal(t, jsonOf(t, bare.Fingerprinted()), jsonOf(t, planned.Fingerprinted()))
}

func TestTheAppliedRecapMarksOnlyTheAnswersTheApplySaved(t *testing.T) {
	req := create.Request{Branches: []string{branchName}, Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation"}}}
	cases := map[string]struct {
		config string
		want   domain.Mark
	}{
		"saved":       {projectToml, domain.MarkSaved},
		"not_created": {withHook("exit 1"), ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, seed{Run: jobsToml, Config: c.config})

			outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{Report: (&operationtest.Recorder{}).Add})

			require.NoError(t, err)
			at := slices.IndexFunc(outcome.Plan.Recap, func(row domain.RecapRow) bool { return row.Key == domain.PathIsolation })
			require.GreaterOrEqual(t, at, 0)
			assert.Equal(t, c.want, outcome.Plan.Recap[at].Mark)
		})
	}
}

func TestARunTomlThatDoesNotParseKeepsTheIsolationRow(t *testing.T) {
	w := newWorld(t, seed{Run: "bogus = [\n"})

	planned, _ := w.plan(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, project.Isolated, planned.Isolation)
	at := slices.IndexFunc(planned.Recap, func(row domain.RecapRow) bool { return row.Key == domain.PathIsolation })
	require.GreaterOrEqual(t, at, 0, planned.Recap)
	assert.Equal(t, []domain.RecapValue{{Value: "isolated", Note: provision.NoteRunUnreadable, Text: "isolated (run.toml unreadable)"}}, planned.Recap[at].Values)
}

func TestARememberedIsolationOverAnUnparsableRunTomlKeepsItsMark(t *testing.T) {
	w := newWorld(t, seed{Run: "bogus = [\n", Config: projectToml + "\n[wizard.remembered]\nisolation = \"verbatim\"\n"})

	planned, _ := w.plan(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, []string{domain.PathIsolation}, planned.Recalled)
	at := slices.IndexFunc(planned.Recap, func(row domain.RecapRow) bool { return row.Key == domain.PathIsolation })
	require.GreaterOrEqual(t, at, 0, planned.Recap)
	assert.Equal(t, domain.RecapRow{Key: domain.PathIsolation, Label: "Isolation", Path: domain.PathIsolation, Mark: domain.MarkRecalled, Values: []domain.RecapValue{{Value: "verbatim", Note: provision.NoteRunUnreadable, Text: "verbatim (run.toml unreadable)"}}}, planned.Recap[at])
}

func TestAnIsolationGivenOverAnUnparsableRunTomlIsRememberedWhenAsked(t *testing.T) {
	w := newWorld(t, seed{Run: "bogus = [\n"})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation"}}})

	assert.Equal(t, []string{"isolation"}, result.Remembered)
	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `isolation = "verbatim"`)
}

func TestAWorktreeWithNoEnvFileStillRecapsItsStrategy(t *testing.T) {
	w := newWorld(t, seed{NoEnv: true})

	planned, _ := w.plan(t, create.Request{Branches: []string{branchName}})

	assert.True(t, slices.ContainsFunc(planned.Recap, func(row domain.RecapRow) bool { return row.Key == domain.PathEnvFrom }), planned.Recap)
}
