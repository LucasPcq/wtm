package app_test

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	envHelperState = "WTM_NEXT_TEST_LOCK_STATE"
	readyLine      = "holding"
)

func TestADryRunTakesNoLockWritesNothingAndPublishesNothing(t *testing.T) {
	w := newWorld(t, seed{})
	held, err := oplock.Locks{StateDir: w.stateDir}.Take(operation.Operation{ID: "other", Locks: []operation.Lock{{Scope: operation.ScopeWorktree, Name: branchName, Mode: operation.Exclusive}}})
	require.NoError(t, err)
	defer held.Release()
	report := &recorder{}

	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, DryRun: true, Report: report.add})

	require.NoError(t, err)
	assert.Equal(t, "op-1", outcome.Op)
	assert.Equal(t, []create.BranchPlan{{Branch: branchName, Path: w.path(branchName), Action: create.ActionCreate}}, outcome.Plan.Branches)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{24}$`), outcome.Fingerprint)
	assert.Nil(t, outcome.Result)
	assert.NoDirExists(t, w.path(branchName))
	assert.NoDirExists(t, filepath.Join(w.stateDir, "worktrees"))
	assert.Empty(t, report.all())
	assert.Empty(t, w.published.all())
}

func TestAWorktreeIsCreatedUnderItsLocksAndAnnouncedFromStartToFinish(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})
	report := &recorder{}

	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, Surface: "tui", Report: report.add})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, wtm.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)
	assert.Equal(t, "2026-10-16T12:00:00Z", outcome.Result.Items[0].Detail.Metadata.CreatedAt)
	events := w.published.all()
	assert.Equal(t, wtm.OperationStarted{Command: "create", Surface: "tui"}, events[0])
	assert.Equal(t, wtm.OperationFinished{Command: "create", Surface: "tui", OK: true}, events[len(events)-1])
	assert.Contains(t, events, wtm.Event(wtm.UnitFinished{Subject: branchName, Status: wtm.StatusDone}))
	assert.True(t, containsKind[wtm.WorktreeCreated](events))
	assert.True(t, containsKind[wtm.WorktreeNumbered](events))
	assert.True(t, containsKind[wtm.WorktreeProvisioned](events))
	assert.Equal(t, events, report.all())
	assert.NoFileExists(t, filepath.Join(w.stateDir, "locks", "ops", "op-1.json"))
}

func TestARunWhoseBranchFailsFinishesNotOkWithItsCode(t *testing.T) {
	w := newWorld(t, seed{Config: projectToml + "\n[hooks]\non_create = [{ cmd = \"exit 3\" }]\n"})

	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, Surface: "cli", Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.Equal(t, wtm.StatusFailed, outcome.Result.Items[0].Status)
	events := w.published.all()
	assert.Equal(t, wtm.OperationFinished{Command: "create", Surface: "cli", Code: "unit.phase_failed"}, events[len(events)-1])
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

	_, refused := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, Report: (&recorder{}).add})

	failure := refusal(t, refused)
	assert.Equal(t, wtm.KindConflict, failure.Kind)
	assert.Equal(t, "lock.held", failure.Code)
	assert.Equal(t, "repo", failure.Params["key"])
	assert.Equal(t, strconv.Itoa(child.Process.Pid), failure.Params["pid"])
	assert.NoDirExists(t, w.path(branchName))
	assert.Empty(t, w.published.all())

	require.NoError(t, child.Process.Kill())
	_ = child.Wait()
	_, err = app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, Report: (&recorder{}).add})
	assert.NoError(t, err, "a dead process still held the repository")
}

func TestHelperProcess(t *testing.T) {
	stateDir := os.Getenv(envHelperState)
	if stateDir == "" {
		t.Skip("run by TestARepositoryHeldByAnotherProcessRefusesTheRunAndChangesNothing")
	}
	held, err := oplock.Locks{StateDir: stateDir}.Take(operation.Operation{ID: "child", PID: os.Getpid(), Command: "create", Locks: []operation.Lock{{Scope: operation.ScopeRepo, Mode: operation.Exclusive}}})
	require.NoError(t, err)
	defer held.Release()
	os.Stdout.WriteString(readyLine + "\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestAPreviewAppliesWhileItsPlanHoldsAndIsRefusedOnceItChanged(t *testing.T) {
	w := newWorld(t, seed{})
	a := app.New(w.deps)
	preview, err := a.Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, DryRun: true, Report: (&recorder{}).add})
	require.NoError(t, err)

	writeFile(t, filepath.Join(w.stateDir, "config.toml"), strings.Replace(projectToml, `strategy = "main"`, `strategy = "example"`, 1))
	stale, staleErr := a.Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, ExpectFingerprint: preview.Fingerprint, Report: (&recorder{}).add})

	failure := refusal(t, staleErr)
	assert.Equal(t, wtm.KindPrecondition, failure.Kind)
	assert.Equal(t, app.CodePlanStale, failure.Code)
	assert.Equal(t, wtm.Params{"expected": preview.Fingerprint, "found": stale.Fingerprint}, failure.Params)
	assert.NotEqual(t, preview.Fingerprint, stale.Fingerprint)
	assert.NoDirExists(t, w.path(branchName))
	assert.Empty(t, w.published.all())
}

func TestAPreviewStillHoldsWhenAnUnrelatedBranchMoved(t *testing.T) {
	w := newWorld(t, seed{})
	a := app.New(w.deps)
	preview, err := a.Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, DryRun: true, Report: (&recorder{}).add})
	require.NoError(t, err)
	gittest.CreateBranch(t, w.main, "unrelated")

	outcome, err := a.Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, ExpectFingerprint: preview.Fingerprint, Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.Equal(t, wtm.StatusDone, outcome.Result.Items[0].Status)
}

func TestALockTheDefaultsNameIsTakenThenTheFactsObservedAgain(t *testing.T) {
	w := newWorld(t, seed{Behind: true, Config: projectToml + "\n[wizard.remembered]\nsource_update = \"ff\"\n"})
	observed := 0
	w.deps.Create.Git = counted{Repo: w.repo, observed: &observed}
	held, err := oplock.Locks{StateDir: w.stateDir}.Take(operation.Operation{ID: "other", Command: "sync", PID: 7, Locks: []operation.Lock{{Scope: operation.ScopeWorktree, Name: "main", Mode: operation.Exclusive}}})
	require.NoError(t, err)

	_, refused := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, Report: (&recorder{}).add})
	held.Release()
	observed = 0
	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, Report: (&recorder{}).add})

	assert.Equal(t, "wt:main", refusal(t, refused).Params["key"])
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, outcome.Plan.FastForward)
	assert.Equal(t, 2, observed)
}

type unprotected struct{}

func (unprotected) Grow([]operation.Lock) (bool, error) { return false, nil }
func (unprotected) Protected() bool                     { return false }
func (unprotected) Release()                            {}

func TestARunWithoutItsLocksSaysSoOnce(t *testing.T) {
	w := newWorld(t, seed{})
	w.deps.TakeLocks = func(operation.Operation) (app.Hold, error) { return unprotected{}, nil }

	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}, Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.Equal(t, []wtm.Warning{{Code: "locks.unavailable", Message: "ran without operation locks"}}, outcome.Warnings)
}

func TestTheWarningsOfThePlanComeWithThoseOfTheRun(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}, From: "feature", EnvFrom: project.EnvParent}, Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.Equal(t, []string{"create.env_parent_fallback"}, codesOf(outcome.Warnings))
}

func TestTheQuestionsAreAskedOfTheBranchesOfTheRequest(t *testing.T) {
	w := newWorld(t, seed{})
	writeFile(t, filepath.Join(w.path(branchName), "work"), "x")

	questions, err := app.New(w.deps).CreateQuestions(t.Context(), create.Request{Branches: []string{branchName}})

	require.NoError(t, err)
	assert.Equal(t, "create.path_exists", questions.Errors[0].Code)
	assert.Equal(t, create.Answer[string]{Value: "main", Origin: create.OriginConfig}, questions.From.Answer)
}

func TestAnInvalidRequestIsRefusedBeforeAnythingIsLockedOrWritten(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{"bad..name"}}, Report: (&recorder{}).add})

	assert.Equal(t, wtm.KindInvalid, refusal(t, err).Kind)
	assert.NoFileExists(t, filepath.Join(w.stateDir, "locks", "ops", "op-1.json"))
	assert.Empty(t, w.published.all())
}

func TestARunWithoutLocksThatRemembersSaysSoOnce(t *testing.T) {
	w := newWorld(t, seed{})
	w.deps.TakeLocks = func(operation.Operation) (app.Hold, error) { return unprotected{}, nil }
	w.deps.Create.Remember = func(context.Context, map[string]string) (wtm.Written, error) {
		return wtm.Written{Changed: true, Unlocked: true}, nil
	}

	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}, Remember: []string{"env_from"}}, Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.Equal(t, []string{"locks.unavailable"}, codesOf(outcome.Warnings))
}

func TestARunWithoutReportStillRuns(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := app.New(w.deps).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{branchName}}})

	require.NoError(t, err)
	assert.Equal(t, wtm.StatusDone, outcome.Result.Items[0].Status)
}

func TestAnOutcomeFailsOnItsFirstFailedItemAndAPreviewNever(t *testing.T) {
	failure := &wtm.Error{Kind: wtm.KindInternal, Code: "unit.phase_failed"}
	applied := app.CreateOutcome{Result: &create.Result{Items: []create.Item{{Subject: branchName, Status: wtm.StatusFailed, Err: failure}}}}

	assert.Same(t, failure, applied.Failure())
	assert.Nil(t, app.CreateOutcome{}.Failure())
}
