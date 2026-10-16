package provision_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
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

func TestALinkedWorktreeWithPortLinksIsNumberedThenSettled(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}

	provision.Run(t.Context(), portLinked(w.params(report)))

	assert.Equal(t, []string{"worktree", "env", "meta", "ordinal", "ports"}, report.phases())
}

func TestAVerbatimWorktreeIsNeitherNumberedNorSettled(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}
	params := portLinked(w.params(report))
	params.Worktree.Isolation = run.Verbatim

	provision.Run(t.Context(), params)

	assert.Equal(t, []string{"worktree", "env", "meta"}, report.phases())
}

func TestARunConfigThatDoesNotLoadStillHasItsPortPassToSaySo(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}
	params := portLinked(w.params(report))
	params.Project.RunErr = errors.New("run.toml: line 3")

	provision.Run(t.Context(), params)

	assert.Equal(t, []string{"worktree", "env", "meta", "ports"}, report.phases())
}

func TestHooksAreNumberedFirstOnlyWhenTheyReadTheRunEnvironment(t *testing.T) {
	plain, composedReport := &recorder{}, &recorder{}

	provision.Run(t.Context(), hooked(onDisk(t).params(plain), "true"))
	provision.Run(t.Context(), hooked(composed(onDisk(t).params(composedReport)), "true"))

	assert.Equal(t, []string{"worktree", "env", "meta", "on_create"}, plain.phases())
	assert.Equal(t, []string{"worktree", "env", "meta", "ordinal", "ports", "on_create"}, composedReport.phases())
}

func TestAWorktreeIsCreatedNumberedThenProvisionedInThatOrder(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := provision.Run(t.Context(), portLinked(w.params(report)))

	require.Equal(t, wtm.StatusDone, outcome.Status, outcome.Err)
	created := worktree.Identity{Branch: branchName, Path: w.path(branchName), Parent: "main", Isolation: run.Isolated, CreatedAt: "2026-10-16T12:00:00Z"}
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
	assert.Equal(t, wtm.WorktreeProvisioned{Worktree: worktree.Identity{Branch: branchName, Path: w.path(branchName), Parent: "main", Isolation: run.Isolated, CreatedAt: "2026-10-16T12:00:00Z"}}, report.worktreeEvents()[1])
}
