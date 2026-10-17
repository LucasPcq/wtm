package provision_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAProjectWithoutRunConfigOrHooksStopsAtItsRecord(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}

	outcome := provision.Run(t.Context(), w.params(report))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta"}, report.phases())
}

func TestAWorktreeIsCreatedNumberedThenProvisionedInThatOrder(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := provision.Run(t.Context(), portLinked(w.params(report)))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	created := worktree.Identity{Branch: branchName, Path: w.path(branchName), Parent: "main", Isolation: project.Isolated, CreatedAt: "2026-10-16T12:00:00Z"}
	numbered := created
	numbered.Ordinal = new(1)
	assert.Equal(t, []wtm.Event{
		wtm.WorktreeCreated{Worktree: created},
		wtm.WorktreeNumbered{Worktree: numbered},
		wtm.WorktreeProvisioned{Worktree: numbered, OK: true},
	}, report.worktreeEvents())
}

func TestEachPhaseIsReportedUnderItsBranch(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}

	provision.Run(t.Context(), w.params(report))

	assert.Equal(t, []wtm.Event{
		wtm.PhaseStarted{Subject: branchName, Phase: "worktree"},
		wtm.PhaseFinished{Subject: branchName, Phase: "worktree"},
		wtm.PhaseStarted{Subject: branchName, Phase: "env"},
		wtm.PhaseFinished{Subject: branchName, Phase: "env"},
		wtm.PhaseStarted{Subject: branchName, Phase: "meta"},
		wtm.PhaseFinished{Subject: branchName, Phase: "meta"},
	}, report.all()[:6])
}

func TestAnInterruptBeforeTheStartCreatesNothing(t *testing.T) {
	w, files := inMemory(t, seeds{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report := &recorder{}

	outcome := provision.Run(ctx, w.params(report))

	assert.Equal(t, wtm.StatusCancelled, outcome.Status)
	assert.Equal(t, wtm.ReasonInterrupted, outcome.Reason)
	assert.Equal(t, wtm.CodeInterrupted, outcome.Err.Code)
	assert.Empty(t, report.all())
	assert.Empty(t, files.Calls())
}

func TestADeadlinePassedBeforeTheStartIsAnInterrupt(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()

	outcome := provision.Run(ctx, w.params(&recorder{}))

	assert.Equal(t, wtm.StatusCancelled, outcome.Status)
	assert.Equal(t, wtm.KindCancelled, outcome.Err.Kind)
	assert.Equal(t, wtm.CodeInterrupted, outcome.Err.Code)
	assert.Equal(t, "interrupted", outcome.Err.Message)
	assert.ErrorIs(t, outcome.Err, context.DeadlineExceeded)
}

func TestTheNumberIsAnnouncedOnceItsPhaseHasFinished(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}

	provision.Run(t.Context(), portLinked(w.params(report)))

	events := report.all()
	finished := slices.Index(events, wtm.Event(wtm.PhaseFinished{Subject: branchName, Phase: "ordinal"}))
	require.Positive(t, finished)
	_, numbered := events[finished+1].(wtm.WorktreeNumbered)
	assert.True(t, numbered, events[finished+1])
}

func TestAnInterruptDuringTheAddFinishesItsEnvAndRecordThenStops(t *testing.T) {
	w := onDisk(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	report := &recorder{on: func(event wtm.Event) {
		if started, ok := event.(wtm.PhaseStarted); ok && started.Phase == "worktree" {
			cancel()
		}
	}}

	outcome := provision.Run(ctx, hooked(portLinked(w.params(report)), "echo never"))

	assert.Equal(t, wtm.StatusCancelled, outcome.Status)
	assert.Equal(t, wtm.ReasonIncomplete, outcome.Reason)
	assert.Equal(t, wtm.CodeInterrupted, outcome.Err.Code)
	assert.Equal(t, []string{"worktree", "env", "meta"}, report.phases())
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Zero(t, recorded.Ordinal)
	assert.Nil(t, outcome.Provisioned.Ordinal)
	assert.Empty(t, report.lines())
	assert.Equal(t, wtm.WorktreeProvisioned{Worktree: worktree.Identity{Branch: branchName, Path: w.path(branchName), Parent: "main", Isolation: project.Isolated, CreatedAt: "2026-10-16T12:00:00Z"}}, report.worktreeEvents()[1])
}

func TestAWorktreeIsAddedWithItsEnvAndItsRecord(t *testing.T) {
	w := onDisk(t)

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	meta := worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvMain, Isolation: project.Isolated}
	assert.Equal(t, provision.Provisioned{Meta: meta}, outcome.Provisioned)
	assert.Empty(t, outcome.Warnings)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Equal(t, meta, recorded)
}

func TestEachEnvThatCouldNotBeCopiedIsAWarning(t *testing.T) {
	w := onDisk(t)
	require.NoError(t, os.Remove(filepath.Join(w.main, ".env")))

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []wtm.Warning{{
		Code:    "provision.env_source_missing",
		Message: "file .env not found in " + w.main + ", skipping",
		Params:  wtm.Params{"target": ".env", "dir": w.main},
	}}, outcome.Warnings)
}

func TestATemplateOrAParentThatIsMissingIsNamed(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	example := w.params(&recorder{})
	example.Project.EnvFiles = []project.EnvFile{{Target: "web/.env"}}
	example.Worktree.Strategy = project.EnvExample
	parent := w.params(&recorder{})
	parent.Worktree.Branch, parent.Worktree.Path = "feat/y", w.path("feat/y")
	parent.Worktree.Checkout = worktree.NewBranch{Name: "feat/y", From: worktree.LocalBranch{Name: "main"}}
	parent.Worktree.Strategy = project.EnvParent
	parent.Worktree.ParentPath = w.path("elsewhere")

	fromTemplate := provision.Run(t.Context(), example)
	fromParent := provision.Run(t.Context(), parent)

	assert.Equal(t, []wtm.Warning{{Code: "provision.env_no_template", Message: "no template found for web/.env, skipping", Params: wtm.Params{"target": "web/.env"}}}, fromTemplate.Warnings)
	assert.Equal(t, []wtm.Warning{{Code: "provision.env_parent_fallback", Message: "file .env not found in the parent worktree, copied from main", Params: wtm.Params{"target": ".env"}}}, fromParent.Warnings)
}

func TestAProjectWithoutEnvFilesCopiesNothing(t *testing.T) {
	w, files := inMemory(t, seeds{})
	params := w.params(&recorder{})
	params.Project.EnvFiles = nil

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.NotContains(t, files.Calls(), "Copy "+w.path(branchName))
}

func TestAWriteWithoutItsLockIsSaidOnce(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	params.Deps.Meta = unlocked{w.meta}

	outcome := provision.Run(t.Context(), params)

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []wtm.Warning{{Code: "locks.unavailable", Message: "ran without operation locks"}}, outcome.Warnings)
}

func TestAnEnvThatCannotBeCopiedLeavesNothingBehind(t *testing.T) {
	for name, params := range map[string]func(world, *recorder) provision.Params{
		"a new branch":       world.params,
		"an existing branch": world.existing,
	} {
		t.Run(name, func(t *testing.T) {
			w := onDisk(t)
			before := gitState(t, w)
			source := filepath.Join(w.main, ".env")
			require.NoError(t, os.Chmod(source, 0o000))
			t.Cleanup(func() { _ = os.Chmod(source, 0o644) })
			report := &recorder{}
			p := params(w, report)

			outcome := provision.Run(t.Context(), p)

			assert.Equal(t, wtm.StatusFailed, outcome.Status)
			assert.Empty(t, outcome.Reason)
			assert.Equal(t, "unit.step_failed", outcome.Err.Code)
			assert.Equal(t, wtm.KindInternal, outcome.Err.Kind)
			assert.Equal(t, wtm.Params{"step": "env"}, outcome.Err.Params)
			assert.ErrorContains(t, outcome.Err, "env failed: read ")
			assert.Equal(t, before, gitState(t, w))
			assert.NoDirExists(t, p.Worktree.Path)
			_, err := w.meta.Load(p.Worktree.Branch)
			assert.ErrorIs(t, err, os.ErrNotExist)
			assert.Empty(t, outcome.Provisioned)
			assert.Empty(t, report.worktreeEvents())
		})
	}
}

func TestARecordThatCannotBeWrittenLeavesNothingBehind(t *testing.T) {
	w, files := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}}})
	before := gitState(t, w)

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, wtm.StatusFailed, outcome.Status)
	assert.Equal(t, "unit.step_failed", outcome.Err.Code)
	assert.Equal(t, "meta failed: disk full", outcome.Err.Error())
	assert.Equal(t, before, gitState(t, w))
	_, recorded := files.Meta(branchName)
	assert.False(t, recorded)
}

func TestAWorktreeThatCannotBeAddedHasNothingToUndo(t *testing.T) {
	w, files := inMemory(t, seeds{GitFail: map[string]error{"AddWorktree": errors.New("path exists")}})

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, wtm.StatusFailed, outcome.Status)
	assert.Equal(t, "worktree failed: path exists", outcome.Err.Error())
	assert.Empty(t, files.Calls())
}

func TestAnUndoThatFailsNamesWhatIsLeftAndTheWayOut(t *testing.T) {
	w, _ := inMemory(t, seeds{
		GitFail: map[string]error{"RemoveWorktree": errors.New("worktree is locked")},
		Store:   storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}},
	})

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, wtm.StatusFailed, outcome.Status)
	assert.Equal(t, "unit.undo_failed", outcome.Err.Code)
	assert.Equal(t, wtm.KindInternal, outcome.Err.Kind)
	assert.Equal(t, "could not remove the worktree, left behind: worktree", outcome.Err.Message)
	assert.Equal(t, wtm.Params{"left": "worktree"}, outcome.Err.Params)
	assert.ErrorContains(t, outcome.Err, "disk full")
	assert.ErrorContains(t, outcome.Err, "worktree is locked")
	followUp := outcome.Err.FollowUp
	require.NotNil(t, followUp)
	assert.Equal(t, "clean", followUp.Command)
	assert.JSONEq(t, `{"branches":["feat/x"],"force":true}`, string(followUp.Request))
	assert.Equal(t, "provision.left_behind", followUp.Code)
	assert.Equal(t, wtm.Params{"branch": branchName, "path": w.path(branchName)}, followUp.Params)
	assert.Equal(t, "branch feat/x is left half-created at "+w.path(branchName)+": remove it with `wtm clean feat/x --force`", followUp.Message)
}

func TestABranchThatCannotBeDeletedIsLeftBehindToo(t *testing.T) {
	w, _ := inMemory(t, seeds{
		GitFail: map[string]error{"DeleteBranch": errors.New("branch is protected")},
		Store:   storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}},
	})

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, "unit.undo_failed", outcome.Err.Code)
	assert.Equal(t, "could not delete the branch, left behind: branch", outcome.Err.Message)
	assert.Equal(t, wtm.Params{"left": "branch"}, outcome.Err.Params)
	assert.ErrorContains(t, outcome.Err, "branch is protected")
	followUp := outcome.Err.FollowUp
	require.NotNil(t, followUp)
	assert.Equal(t, "clean", followUp.Command)
	assert.JSONEq(t, `{"branches":["feat/x"],"force":true}`, string(followUp.Request))
	assert.Equal(t, "provision.left_behind", followUp.Code)
	assert.Equal(t, wtm.Params{"branch": branchName}, followUp.Params)
	assert.Equal(t, "branch feat/x is left behind: remove it with `wtm clean feat/x --force`", followUp.Message)
}

func TestACheckOutNeverDeletesTheBranchOnUndo(t *testing.T) {
	w, _ := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}}})

	outcome := provision.Run(t.Context(), w.existing(&recorder{}))

	assert.Equal(t, "unit.step_failed", outcome.Err.Code)
	assert.Contains(t, gitStateNames(t, w), existing)
}
