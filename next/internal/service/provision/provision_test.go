package provision_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAProjectWithoutRunConfigOrHooksStopsAtItsRecord(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}

	outcome := w.service.Run(t.Context(), w.params(report))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []string{"worktree", "env", "meta"}, report.phases())
}

func TestAWorktreeIsCreatedNumberedThenProvisionedInThatOrder(t *testing.T) {
	w := onDisk(t)
	report := &recorder{}

	outcome := w.service.Run(t.Context(), portLinked(w.params(report)))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	created := worktree.Identity{Branch: branchName, Path: w.path(branchName), Parent: "main", Isolation: project.Isolated, CreatedAt: "2026-10-16T12:00:00Z"}
	numbered := created
	numbered.Ordinal = new(1)
	assert.Equal(t, []domain.Event{
		domain.WorktreeCreated{Worktree: created},
		domain.WorktreeNumbered{Worktree: numbered},
		domain.WorktreeProvisioned{Worktree: numbered, OK: true},
	}, report.worktreeEvents())
}

func TestEachPhaseIsReportedUnderItsBranch(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}

	w.service.Run(t.Context(), w.params(report))

	assert.Equal(t, []domain.Event{
		domain.PhaseStarted{Subject: branchName, Phase: "worktree"},
		domain.PhaseFinished{Subject: branchName, Phase: "worktree"},
		domain.PhaseStarted{Subject: branchName, Phase: "env"},
		domain.PhaseFinished{Subject: branchName, Phase: "env"},
		domain.PhaseStarted{Subject: branchName, Phase: "meta"},
		domain.PhaseFinished{Subject: branchName, Phase: "meta"},
	}, report.all()[:6])
}

func TestAnInterruptBeforeTheStartCreatesNothing(t *testing.T) {
	w, files := inMemory(t, seeds{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report := &recorder{}

	outcome := w.service.Run(ctx, w.params(report))

	assert.Equal(t, domain.StatusCancelled, outcome.Status)
	assert.Equal(t, domain.ReasonInterrupted, outcome.Reason)
	assert.Equal(t, domain.CodeInterrupted, outcome.Err.Code)
	assert.Empty(t, report.all())
	assert.Empty(t, files.Calls())
}

func TestADeadlinePassedBeforeTheStartIsAnInterrupt(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()

	outcome := w.service.Run(ctx, w.params(&recorder{}))

	assert.Equal(t, domain.StatusCancelled, outcome.Status)
	assert.Equal(t, domain.KindCancelled, outcome.Err.Kind)
	assert.Equal(t, domain.CodeInterrupted, outcome.Err.Code)
	assert.Equal(t, "interrupted", outcome.Err.Message)
	assert.ErrorIs(t, outcome.Err, context.DeadlineExceeded)
}

func TestTheNumberIsAnnouncedOnceItsPhaseHasFinished(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	report := &recorder{}

	w.service.Run(t.Context(), portLinked(w.params(report)))

	events := report.all()
	finished := slices.Index(events, domain.Event(domain.PhaseFinished{Subject: branchName, Phase: "ordinal"}))
	require.Positive(t, finished)
	_, numbered := events[finished+1].(domain.WorktreeNumbered)
	assert.True(t, numbered, events[finished+1])
}

func TestAnInterruptDuringTheAddFinishesItsEnvAndRecordThenStops(t *testing.T) {
	w := onDisk(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	report := &recorder{on: func(event domain.Event) {
		if started, ok := event.(domain.PhaseStarted); ok && started.Phase == "worktree" {
			cancel()
		}
	}}

	outcome := w.service.Run(ctx, hooked(portLinked(w.params(report)), "echo never"))

	assert.Equal(t, domain.StatusCancelled, outcome.Status)
	assert.Equal(t, domain.ReasonIncomplete, outcome.Reason)
	assert.Equal(t, domain.CodeInterrupted, outcome.Err.Code)
	assert.Equal(t, []string{"worktree", "env", "meta"}, report.phases())
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branchName), ".env")))
	recorded, err := w.meta.Load(branchName)
	require.NoError(t, err)
	assert.Zero(t, recorded.Ordinal)
	assert.Zero(t, outcome.Provisioned.Meta.Ordinal)
	assert.Empty(t, report.lines())
	assert.Equal(t, domain.WorktreeProvisioned{Worktree: worktree.Identity{Branch: branchName, Path: w.path(branchName), Parent: "main", Isolation: project.Isolated, CreatedAt: "2026-10-16T12:00:00Z"}}, report.worktreeEvents()[1])
}

func TestAWorktreeIsAddedWithItsEnvAndItsRecord(t *testing.T) {
	w := onDisk(t)

	outcome := w.service.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
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

	outcome := w.service.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []domain.Warning{{
		Code:    "provision.env_source_missing",
		Message: "file .env not found in " + w.main + ", skipping",
		Params:  domain.Params{"target": ".env", "dir": w.main},
	}}, outcome.Warnings)
}

func TestATemplateOrAParentThatIsMissingIsNamed(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	example := w.params(&recorder{})
	example.Project.EnvFiles = []project.EnvFile{{Target: "web/.env"}}
	example.Worktree.Answers = answered(provision.Given{EnvFrom: project.EnvExample})
	parent := w.params(&recorder{})
	parent.Worktree.Branch, parent.Worktree.Path = "feat/y", w.path("feat/y")
	parent.Worktree.Checkout = worktree.NewBranch{Name: "feat/y", From: worktree.LocalBranch{Name: "main"}}
	parent.Worktree.Answers = answered(provision.Given{EnvFrom: project.EnvParent})
	parent.Worktree.ParentPath = w.path("elsewhere")

	fromTemplate := w.service.Run(t.Context(), example)
	fromParent := w.service.Run(t.Context(), parent)

	assert.Equal(t, []domain.Warning{{Code: "provision.env_no_template", Message: "no template found for web/.env, skipping", Params: domain.Params{"target": "web/.env"}}}, fromTemplate.Warnings)
	assert.Equal(t, []domain.Warning{{Code: "provision.env_parent_fallback", Message: "file .env not found in the parent worktree, copied from main", Params: domain.Params{"target": ".env"}}}, fromParent.Warnings)
}

func TestAProjectWithoutEnvFilesCopiesNothing(t *testing.T) {
	w, files := inMemory(t, seeds{})
	params := w.params(&recorder{})
	params.Project.EnvFiles = nil

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.NotContains(t, files.Calls(), "Copy "+w.path(branchName))
}

func TestAWriteWithoutItsLockIsSaidOnce(t *testing.T) {
	w := onDisk(t)
	params := portLinked(w.params(&recorder{}))
	w.service.Meta = unlocked{w.meta}

	outcome := w.service.Run(t.Context(), params)

	require.Equal(t, domain.StatusDone, outcome.Status, outcome.Err)
	assert.Equal(t, []domain.Warning{{Code: "locks.unavailable", Message: "ran without operation locks"}}, outcome.Warnings)
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

			outcome := w.service.Run(t.Context(), p)

			assert.Equal(t, domain.StatusFailed, outcome.Status)
			assert.Empty(t, outcome.Reason)
			assert.Equal(t, "unit.step_failed", outcome.Err.Code)
			assert.Equal(t, domain.KindInternal, outcome.Err.Kind)
			assert.Equal(t, domain.Params{"step": "env"}, outcome.Err.Params)
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

	outcome := w.service.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, domain.StatusFailed, outcome.Status)
	assert.Equal(t, "unit.step_failed", outcome.Err.Code)
	assert.Equal(t, "meta failed: disk full", outcome.Err.Error())
	assert.Equal(t, before, gitState(t, w))
	_, recorded := files.Meta(branchName)
	assert.False(t, recorded)
}

func TestAWorktreeThatCannotBeAddedHasNothingToUndo(t *testing.T) {
	w, files := inMemory(t, seeds{GitFail: map[string]error{"AddWorktree": errors.New("path exists")}})

	outcome := w.service.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, domain.StatusFailed, outcome.Status)
	assert.Equal(t, "worktree failed: path exists", outcome.Err.Error())
	assert.Empty(t, files.Calls())
}

func TestAnUndoThatFailsNamesWhatIsLeftAndTheWayOut(t *testing.T) {
	w, _ := inMemory(t, seeds{
		GitFail: map[string]error{"RemoveWorktree": errors.New("worktree is locked")},
		Store:   storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}},
	})

	outcome := w.service.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, domain.StatusFailed, outcome.Status)
	assert.Equal(t, "unit.undo_failed", outcome.Err.Code)
	assert.Equal(t, domain.KindInternal, outcome.Err.Kind)
	assert.Equal(t, "could not remove the worktree, left behind: worktree", outcome.Err.Message)
	assert.Equal(t, domain.Params{"left": "worktree"}, outcome.Err.Params)
	assert.ErrorContains(t, outcome.Err, "disk full")
	assert.ErrorContains(t, outcome.Err, "worktree is locked")
	followUp := outcome.Err.FollowUp
	require.NotNil(t, followUp)
	assert.Equal(t, []string{"remove", "feat/x"}, followUp.Argv)
	assert.Equal(t, "provision.left_behind", followUp.Code)
	assert.Equal(t, domain.Params{"branch": branchName, "path": w.path(branchName)}, followUp.Params)
	assert.Equal(t, "branch feat/x is left half-created at "+w.path(branchName), followUp.Message)
}

func TestABranchThatCannotBeDeletedIsLeftBehindToo(t *testing.T) {
	w, _ := inMemory(t, seeds{
		GitFail: map[string]error{"DeleteBranch": errors.New("branch is protected")},
		Store:   storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}},
	})

	outcome := w.service.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, "unit.undo_failed", outcome.Err.Code)
	assert.Equal(t, "could not delete the branch, left behind: branch", outcome.Err.Message)
	assert.Equal(t, domain.Params{"left": "branch"}, outcome.Err.Params)
	assert.ErrorContains(t, outcome.Err, "branch is protected")
	followUp := outcome.Err.FollowUp
	require.NotNil(t, followUp)
	assert.Equal(t, []string{"remove", "feat/x"}, followUp.Argv)
	assert.Equal(t, "provision.left_behind", followUp.Code)
	assert.Equal(t, domain.Params{"branch": branchName}, followUp.Params)
	assert.Equal(t, "branch feat/x is left behind", followUp.Message)
}

func TestACheckOutNeverDeletesTheBranchOnUndo(t *testing.T) {
	w, _ := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}}})

	outcome := w.service.Run(t.Context(), w.existing(&recorder{}))

	assert.Equal(t, "unit.step_failed", outcome.Err.Code)
	assert.Contains(t, gitStateNames(t, w), existing)
}

func TestTheProjectProvisionedIsTheOneObserved(t *testing.T) {
	runErr := errors.New("bad run.toml")
	repo := worktree.Repository{
		MainPath: "/repo/main",
		Config:   project.Config{Env: project.Env{Files: []project.EnvFile{{Target: ".env"}}}, OnCreate: []project.Hook{{Cmd: "make"}}},
		Global:   project.Global{Proxy: project.Proxy{Port: 8080}},
		Run:      project.Run{Isolation: project.Verbatim},
		RunErr:   runErr,
	}

	assert.Equal(t, provision.Project{
		MainPath: "/repo/main",
		EnvFiles: []project.EnvFile{{Target: ".env"}},
		OnCreate: []project.Hook{{Cmd: "make"}},
		Run:      project.Run{Isolation: project.Verbatim},
		RunErr:   runErr,
		Global:   project.Global{Proxy: project.Proxy{Port: 8080}},
	}, provision.ParamsOf(repo, provision.Planned{}, nil).Project)
}

func TestParamsOfCheckOutFromTheRepository(t *testing.T) {
	repo := worktree.Repository{MainPath: "/repo/main", Snapshot: worktree.Snapshot{Worktrees: []worktree.Worktree{{Path: "/repo/main", State: worktree.OnBranch{Branch: "main"}}}}}
	answers := answered(provision.Given{})

	params := provision.ParamsOf(repo, provision.Planned{Branch: branchName, Path: "/trees/x", Placement: worktree.Placed{Start: worktree.StartCreate}, From: "main", Answers: answers}, nil)

	assert.Equal(t, provision.Worktree{
		Branch:     branchName,
		Path:       "/trees/x",
		Checkout:   worktree.NewBranch{Name: branchName, From: worktree.LocalBranch{Name: "main"}},
		Source:     "main",
		ParentPath: "/repo/main",
		Answers:    answers,
	}, params.Worktree)
}

func TestTheFingerprintCoversTheEnvFilesButNotTheConfiguredStrategy(t *testing.T) {
	fingerprint := func(repo worktree.Repository) string {
		data, err := json.Marshal(provision.Fingerprinted("plan", repo, answered(provision.Given{})))
		require.NoError(t, err)
		return string(data)
	}
	repo := repository(nil)
	before := fingerprint(repo)

	repo.Config.Env.Strategy = project.EnvParent
	assert.Equal(t, before, fingerprint(repo))

	repo.Config.Env.Files = []project.EnvFile{{Target: ".env"}}
	assert.NotEqual(t, before, fingerprint(repo))
}

func TestARememberThatFailsIsTheCommandsWarning(t *testing.T) {
	service := provision.Service{Answers: storetest.NewFake(storetest.FakeParams{Fail: map[string]error{"Remember": errors.New("read-only file system")}})}

	remembered, warnings := service.Remember(t.Context(), "greet", answered(provision.Given{EnvFrom: project.EnvMain}), []string{domain.PathEnvFrom})

	assert.Empty(t, remembered)
	require.Len(t, warnings, 1)
	assert.Equal(t, "greet.remember_failed", warnings[0].Code)
	assert.Equal(t, domain.Params{"cause": "read-only file system"}, warnings[0].Params)
}

func TestARememberReturnsTheFieldsItWrote(t *testing.T) {
	service := provision.Service{Answers: storetest.NewFake(storetest.FakeParams{Config: &project.Config{}})}

	remembered, warnings := service.Remember(t.Context(), "greet", answered(provision.Given{EnvFrom: project.EnvMain}), []string{domain.PathEnvFrom, "keep"})

	assert.Equal(t, []string{domain.PathEnvFrom}, remembered)
	assert.Empty(t, warnings)
}

func TestNothingToRememberWritesNothing(t *testing.T) {
	service := provision.Service{Answers: storetest.NewFake(storetest.FakeParams{Fail: map[string]error{"Remember": errors.New("read-only file system")}})}

	remembered, warnings := service.Remember(t.Context(), "greet", answered(provision.Given{EnvFrom: project.EnvMain}), nil)

	assert.Empty(t, remembered)
	assert.Empty(t, warnings)
}
