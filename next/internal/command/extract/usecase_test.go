package extract_test

import (
	"context"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheSelectedChangesMoveToAnExistingWorktree(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	w.change(t, source, "b.txt", "b\n")

	result, _ := w.apply(t, moving("a.txt"))

	require.Equal(t, domain.StatusDone, result.Item.Status, result.Item.Err)
	assert.Equal(t, &extract.Detail{
		Path:      w.at(other),
		Files:     []worktree.Change{{Path: "a.txt", Status: worktree.ChangeUntracked}},
		Conflicts: []string{},
		Isolation: project.Isolated,
	}, result.Item.Detail)
	assert.Equal(t, "a\n", readFile(t, filepath.Join(w.at(other), "a.txt")))
	assert.NoFileExists(t, filepath.Join(w.at(source), "a.txt"))
	assert.FileExists(t, filepath.Join(w.at(source), "b.txt"))
}

func TestANewTargetIsProvisionedUnderItsLocksThenReceivesTheChanges(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	report := &recorder{}

	outcome, err := w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, operation.Options{Surface: "cli", Report: report.add})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	require.Equal(t, domain.StatusDone, outcome.Result.Item.Status, outcome.Result.Item.Err)
	assert.Equal(t, extract.Target{Branch: split, Path: w.path(split), Action: extract.ActionCreate}, outcome.Plan.Target)
	assert.Equal(t, "a\n", readFile(t, filepath.Join(w.path(split), "a.txt")))
	assert.NoFileExists(t, filepath.Join(w.at(source), "a.txt"))
	recorded, err := w.meta.Load(split)
	require.NoError(t, err)
	assert.Equal(t, "main", recorded.SourceBranch)
	events := w.published.all()
	assert.Equal(t, domain.OperationStarted{Command: "extract", Surface: "cli"}, events[0])
	assert.Equal(t, domain.OperationFinished{Command: "extract", Surface: "cli", OK: true}, events[len(events)-1])
	assert.True(t, containsKind[domain.WorktreeCreated](events))
	assert.True(t, containsKind[domain.WorktreeProvisioned](events))
	assert.Contains(t, events, domain.Event(domain.PhaseStarted{Subject: split, Phase: "move"}))
	assert.Equal(t, events, report.all())
	assert.NoFileExists(t, filepath.Join(w.stateDir, "locks", "ops", "op-1.json"))
}

func TestATargetHeldByAnotherOperationRefusesTheRunAndChangesNothing(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	held, err := oplock.Locks{StateDir: w.stateDir}.Take(domain.Operation{ID: "other", Locks: []domain.Lock{{Scope: domain.ScopeWorktree, Name: split, Mode: domain.LockExclusive}}})
	require.NoError(t, err)
	defer held.Release()

	_, err = w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, operation.Options{Report: (&recorder{}).add})

	require.Error(t, err)
	assert.NoDirExists(t, w.path(split))
	assert.FileExists(t, filepath.Join(w.at(source), "a.txt"))
}

func TestKeepCopiesTheChangesAndLeavesTheSourceAsItWas(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := moving("a.txt")
	req.Keep = new(true)

	result, _ := w.apply(t, req)

	assert.True(t, result.Item.Detail.Kept)
	assert.Equal(t, "a\n", readFile(t, filepath.Join(w.at(other), "a.txt")))
	assert.Equal(t, "a\n", readFile(t, filepath.Join(w.at(source), "a.txt")))
}

func TestResolveWritesConflictMarkersInTheTargetAndKeepsTheSource(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "source\n")
	w.change(t, other, "a.txt", "target\n")
	req := moving("a.txt")
	req.OnConflict = extract.OnConflictResolve

	result, _ := w.apply(t, req)

	require.Equal(t, domain.StatusDone, result.Item.Status, result.Item.Err)
	assert.Equal(t, []string{"a.txt"}, result.Item.Detail.Conflicts)
	assert.True(t, result.Item.Detail.Kept)
	assert.Contains(t, readFile(t, filepath.Join(w.at(other), "a.txt")), ">>>>>>> incoming ("+source+")")
	assert.Equal(t, "source\n", readFile(t, filepath.Join(w.at(source), "a.txt")))
}

func TestAConflictUnderAbortIsRefusedAtPlanTimeAndNothingIsCreatedOrMoved(t *testing.T) {
	w := newWorld(t, seed{})
	writeFile(t, filepath.Join(w.main, "a.txt"), "main\n")
	gittest.Git(t, w.main, "add", "a.txt")
	gittest.Git(t, w.main, "commit", "-q", "-m", "a on main")
	w.change(t, source, "a.txt", "source\n")

	_, err := w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, operation.Options{Report: (&recorder{}).add})

	failure := failureOf(t, err)
	assert.Equal(t, domain.KindConflict, failure.Kind)
	assert.Equal(t, extract.CodeConflict, failure.Code)
	assert.Equal(t, `cannot apply the selected changes: a.txt already exists in "feat/split" — resolve it there, extract to another worktree, or re-run with --on-conflict resolve`, failure.Message)
	assert.NoDirExists(t, w.path(split))
	assert.Equal(t, "source\n", readFile(t, filepath.Join(w.at(source), "a.txt")))
	assert.Empty(t, w.published.all())
}

func TestASourceWithNothingToExtractIsAPlanWithNoFilesAndNoRun(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), extract.Request{Source: source}, operation.Options{Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.True(t, outcome.Plan.Nothing())
	assert.Nil(t, outcome.Result)
	assert.JSONEq(t, `{"source":"feat/login","files":[],"keep":false,"conflicts":[]}`, jsonOf(t, outcome.Plan))
	assert.Empty(t, w.published.all())
}

func TestADryRunMovesNothingAndFingerprintsThePlan(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	outcome, err := w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, operation.Options{DryRun: true, Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{24}$`), outcome.Fingerprint)
	assert.Nil(t, outcome.Result)
	assert.NoDirExists(t, w.path(split))
	assert.FileExists(t, filepath.Join(w.at(source), "a.txt"))
	assert.Empty(t, w.published.all())
}

func TestAPreviewIsRefusedOnceTheChangesItSelectedChanged(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "dir/a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"dir"}, To: other}
	preview, err := w.usecase.Run(t.Context(), req, operation.Options{DryRun: true, Report: (&recorder{}).add})
	require.NoError(t, err)
	w.change(t, source, "dir/b.txt", "b\n")

	_, err = w.usecase.Run(t.Context(), req, operation.Options{ExpectFingerprint: preview.Fingerprint, Report: (&recorder{}).add})

	assert.Equal(t, operation.CodePlanStale, failureOf(t, err).Code)
	assert.NoFileExists(t, filepath.Join(w.at(other), "dir", "a.txt"))
}

func TestAnInterruptDuringTheMoveLetsItFinish(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var seen error
	w.usecase.Git = interrupting{Repo: w.repo, cancel: cancel, seen: &seen}

	result, _ := w.applyIn(ctx, t, moving("a.txt"))

	require.NoError(t, seen)
	assert.Equal(t, domain.StatusDone, result.Item.Status, result.Item.Err)
	assert.FileExists(t, filepath.Join(w.at(other), "a.txt"))
	assert.NoFileExists(t, filepath.Join(w.at(source), "a.txt"))
}

func TestABehindSubjectIsFastForwardedBeforeTheTargetStartsFromIt(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	w.change(t, source, "a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split, FastForward: new(true)}
	planned, _ := w.plan(t, req)

	result, report := w.apply(t, req)

	assert.Equal(t, []domain.Lock{
		{Scope: domain.ScopeRepo, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: source, Mode: domain.LockExclusive},
		{Scope: domain.ScopeWorktree, Name: split, Mode: domain.LockExclusive},
		{Scope: domain.ScopeWorktree, Name: "main", Mode: domain.LockExclusive},
	}, planned.Locks())
	assert.Empty(t, result.Warnings)
	upstream := gittest.Output(t, w.main, "rev-parse", "origin/main")
	assert.Equal(t, upstream, gittest.Output(t, w.path(split), "rev-parse", "HEAD"))
	assert.Equal(t, []domain.Event{
		domain.PhaseStarted{Subject: "main", Phase: "fast_forward"},
		domain.PhaseFinished{Subject: "main", Phase: "fast_forward"},
	}, report.all()[:2])
}

func TestTheAnswersAskedToBeRememberedAreKeptOnceTheTargetIsCreated(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})
	w.change(t, source, "a.txt", "a\n")

	result, _ := w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Isolation: project.Verbatim, Remember: []string{"isolation", "fast_forward"}})

	assert.Empty(t, result.Warnings)
	assert.Equal(t, project.Verbatim, result.Item.Detail.Isolation)
	config := readFile(t, filepath.Join(w.stateDir, "config.toml"))
	assert.Contains(t, config, `isolation = "verbatim"`)
	assert.NotContains(t, config, "source_update")
}

func TestNothingIsRememberedWhenTheTargetAlreadyExisted(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})
	w.change(t, source, "a.txt", "a\n")
	req := moving("a.txt")
	req.Remember = []string{"isolation"}

	w.apply(t, req)

	assert.NotContains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), "isolation")
}

func TestAMoveRefusedSinceThePlanIsAConflictOfTheRun(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "source\n")
	planned, _ := w.plan(t, moving("a.txt"))
	w.change(t, other, "a.txt", "target\n")

	result := w.usecase.Apply(t.Context(), planned, (&recorder{}).add)

	assert.Equal(t, domain.StatusFailed, result.Item.Status)
	assert.Equal(t, extract.CodeConflict, result.Item.Err.Code)
	assert.Equal(t, result.Item.Err, result.Failure())
	assert.Equal(t, "source\n", readFile(t, filepath.Join(w.at(source), "a.txt")))
}
