package extract_test

import (
	"context"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

func TestTheSelectedChangesMoveToAnExistingWorktree(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	w.change(t, source, "b.txt", "b\n")

	result, _ := w.apply(t, moving("a.txt"))

	require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	assert.Equal(t, &extract.Detail{
		Path:      w.at(other),
		Files:     []worktree.Change{{Path: "a.txt", Status: worktree.ChangeUntracked}},
		Conflicts: []string{},
		Isolation: project.Isolated,
	}, result.Items[0].Detail)
	assert.Equal(t, "a\n", readFile(t, filepath.Join(w.at(other), "a.txt")))
	assert.NoFileExists(t, filepath.Join(w.at(source), "a.txt"))
	assert.FileExists(t, filepath.Join(w.at(source), "b.txt"))
}

func TestANewTargetIsProvisionedUnderItsLocksThenReceivesTheChanges(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	outcome, err := w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, domain.RunOptions{Surface: "cli"})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	require.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)
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
	assert.NoFileExists(t, filepath.Join(w.stateDir, "locks", "ops", "op-1.json"))
}

func TestATargetHeldByAnotherOperationRefusesTheRunAndChangesNothing(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	held, err := oplock.Locks{StateDir: w.stateDir}.Take(domain.Operation{ID: "other", Locks: []domain.Lock{{Scope: domain.ScopeWorktree, Name: split, Mode: domain.LockExclusive}}})
	require.NoError(t, err)
	defer held.Release()

	_, err = w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, domain.RunOptions{Report: (&recorder{}).add})

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

	assert.True(t, result.Items[0].Detail.Kept)
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

	require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	assert.Equal(t, []string{"a.txt"}, result.Items[0].Detail.Conflicts)
	assert.True(t, result.Items[0].Detail.Kept)
	assert.Contains(t, readFile(t, filepath.Join(w.at(other), "a.txt")), ">>>>>>> incoming ("+source+")")
	assert.Equal(t, "source\n", readFile(t, filepath.Join(w.at(source), "a.txt")))
}

func TestAConflictUnderAbortIsRefusedAtPlanTimeAndNothingIsCreatedOrMoved(t *testing.T) {
	w := newWorld(t, seed{})
	writeFile(t, filepath.Join(w.main, "a.txt"), "main\n")
	gittest.Git(t, w.main, "add", "a.txt")
	gittest.Git(t, w.main, "commit", "-q", "-m", "a on main")
	w.change(t, source, "a.txt", "source\n")

	_, err := w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, domain.RunOptions{Report: (&recorder{}).add})

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

	outcome, err := w.usecase.Run(t.Context(), extract.Request{Source: source}, domain.RunOptions{Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.True(t, outcome.Plan.Nothing())
	assert.Nil(t, outcome.Result)
	assert.JSONEq(t, `{"source":"feat/login","files":[],"keep":false,"conflicts":[]}`, jsonOf(t, outcome.Plan))
	assert.Empty(t, w.published.all())
}

func TestADryRunMovesNothingAndFingerprintsThePlan(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	outcome, err := w.usecase.Run(t.Context(), extract.Request{Source: source, Files: []string{"a.txt"}, To: split}, domain.RunOptions{DryRun: true, Report: (&recorder{}).add})

	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{24}$`), outcome.Fingerprint)
	assert.Nil(t, outcome.Result)
	assert.NoDirExists(t, w.path(split))
	assert.FileExists(t, filepath.Join(w.at(source), "a.txt"))
}

func TestAPreviewIsRefusedOnceTheChangesItSelectedChanged(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "dir/a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"dir"}, To: other}
	preview, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true, Report: (&recorder{}).add})
	require.NoError(t, err)
	w.change(t, source, "dir/b.txt", "b\n")

	_, err = w.usecase.Run(t.Context(), req, domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: (&recorder{}).add})

	assert.Equal(t, domain.CodePlanStale, failureOf(t, err).Code)
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
	assert.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
	assert.FileExists(t, filepath.Join(w.at(other), "a.txt"))
	assert.NoFileExists(t, filepath.Join(w.at(source), "a.txt"))
}

func TestABehindSubjectIsFastForwardedBeforeTheTargetStartsFromIt(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	w.change(t, source, "a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{FastForward: new(true)}}
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

	result, _ := w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation", "fast_forward"}}})

	assert.Empty(t, result.Warnings)
	assert.Equal(t, project.Verbatim, result.Items[0].Detail.Isolation)
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

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Equal(t, extract.CodeConflict, result.Items[0].Err.Code)
	assert.Equal(t, result.Items[0].Err, result.Failure())
	assert.Equal(t, "source\n", readFile(t, filepath.Join(w.at(source), "a.txt")))
}

func TestARefTheApplyFastForwardsIsCheckedForConflictsAsOriginHasIt(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	w.change(t, source, "a.txt", "a\n")
	behindOrigin(t, w, existing)
	var checks []worktree.ConflictCheck
	w.usecase.Git = checking{Repo: w.repo, checks: &checks}

	w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{FastForward: new(true)}})
	w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{FastForward: new(false)}})
	w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: existing, From: "main", Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []string{"origin/main", "main", "origin/" + existing}, targetRefs(checks))
}

func TestNothingIsRememberedNorMovedWhenTheTargetFailsToProvision(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Config: withHook("exit 1")})
	w.change(t, source, "a.txt", "a\n")

	result, _ := w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation"}}})

	require.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.NotContains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), "isolation")
	assert.FileExists(t, filepath.Join(w.at(source), "a.txt"))
	assert.NoFileExists(t, filepath.Join(w.path(split), "a.txt"))
}

func TestNothingIsRememberedWhenTheMoveIsRefused(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})
	w.change(t, source, "a.txt", "a\n")
	w.usecase.Git = halfMoving{Repo: w.repo}

	result, _ := w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{Isolation: project.Verbatim, Remember: []string{"isolation"}}})

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
			writeFile(t, filepath.Join(w.stateDir, "config.toml"), projectToml+"\n[[env.file]]\ntarget = \".env\"\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, seed{Run: jobsToml})
			w.change(t, source, "a.txt", "a\n")
			req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split}
			preview, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true, Report: (&recorder{}).add})
			require.NoError(t, err)
			change(w)

			_, err = w.usecase.Run(t.Context(), req, domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: (&recorder{}).add})

			assert.Equal(t, domain.CodePlanStale, failureOf(t, err).Code)
			assert.NoDirExists(t, w.path(split))
		})
	}
}

func TestAPreviewOfChangesCommittedSinceIsStaleRatherThanNothingToExtract(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := moving("a.txt")
	preview, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true, Report: (&recorder{}).add})
	require.NoError(t, err)
	gittest.Git(t, w.at(source), "add", "a.txt")
	gittest.Git(t, w.at(source), "commit", "-q", "-m", "a")

	_, err = w.usecase.Run(t.Context(), req, domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: (&recorder{}).add})

	assert.Equal(t, domain.CodePlanStale, failureOf(t, err).Code)
}

func TestAMoveThatLeavesTheChangesInBothWorktreesIsHalfMoved(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	w.usecase.Git = halfMoving{Repo: w.repo}

	result, _ := w.apply(t, moving("a.txt"))

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Equal(t, extract.CodeHalfMoved, result.Items[0].Err.Code)
	assert.Equal(t, `the changes are in "feat/other" but "feat/login" still holds them`, result.Items[0].Err.Message)
	assert.Equal(t, result.Items[0].Err, result.Failure())
}

func TestACancelOnceTheTargetStandsSkipsTheMove(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	planned, _ := w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	result := w.usecase.Apply(ctx, planned, func(event domain.Event) {
		if _, provisioned := event.(domain.WorktreeProvisioned); provisioned {
			cancel()
		}
	})

	assert.Equal(t, domain.StatusCancelled, result.Items[0].Status)
	assert.Equal(t, domain.ReasonInterrupted, result.Items[0].Reason)
	assert.Equal(t, domain.CodeInterrupted, result.Failure().Code)
	assert.DirExists(t, w.path(split))
	assert.NoFileExists(t, filepath.Join(w.path(split), "a.txt"))
	assert.FileExists(t, filepath.Join(w.at(source), "a.txt"))
}

func TestASourceWithUncommittedChangesIsNotFastForwardedAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	w.change(t, source, "a.txt", "a\n")
	writeFile(t, filepath.Join(w.main, "dirty"), "dirty\n")

	result, _ := w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []domain.Warning{{
		Code:    "extract.ff_dirty",
		Message: "couldn't fast-forward main: its worktree has uncommitted changes — the target starts from your local branch",
		Params:  domain.Params{"branch": "main"},
	}}, result.Warnings)
	assert.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
}

func TestAFastForwardThatFailsIsAWarningAndTheTargetStillStarts(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	behindOrigin(t, w, existing)
	gittest.Git(t, w.main, "remote", "set-url", "origin", w.root+"/gone")

	result, _ := w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, From: existing, Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []string{"extract.ff_failed"}, codes(result.Warnings))
	assert.Contains(t, result.Warnings[0].Message, "couldn't fast-forward feature: ")
	assert.Contains(t, result.Warnings[0].Message, " — the target starts from your local branch")
	assert.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
}

func TestASourceInTheMiddleOfARebaseIsNotFastForwardedAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	behindOrigin(t, w, existing)
	rebasing(t, w, existing)

	result, _ := w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, From: existing, Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []domain.Warning{{
		Code:    "extract.ff_busy",
		Message: "couldn't fast-forward feature: its worktree is in the middle of a rebase or a bisect — the target starts from your local branch",
		Params:  domain.Params{"branch": existing},
	}}, result.Warnings)
	assert.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
}

func TestACreatedTargetGetsItsEnvByTheStrategyAsked(t *testing.T) {
	cases := map[string]struct {
		config string
		given  project.EnvStrategy
		want   string
	}{
		"config.toml's strategy by default":             {config: envToml, want: exampleEnv},
		"the strategy given":                            {config: envToml, given: project.EnvMain, want: mainEnv},
		"the strategy remembered":                       {config: envToml + "\n[wizard.remembered]\nenv_strategy = \"main\"\n", want: mainEnv},
		"the config default over a remembered strategy": {config: envToml + "\n[wizard.remembered]\nenv_strategy = \"main\"\n", given: provision.EnvFromConfig, want: exampleEnv},
		"the config default remembered":                 {config: envToml + "\n[wizard.remembered]\nenv_strategy = \"config\"\n", want: exampleEnv},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := withEnvFiles(t, c.config)
			req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{EnvFrom: c.given}}

			result, _ := w.apply(t, req)

			require.Equal(t, domain.StatusDone, result.Items[0].Status, result.Items[0].Err)
			assert.Equal(t, c.want, readFile(t, filepath.Join(w.path(split), ".env")))
		})
	}
}

func TestTheEnvStrategyAskedToBeRememberedIsKeptOnceTheTargetIsCreated(t *testing.T) {
	w := withEnvFiles(t, envToml)

	w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{EnvFrom: project.EnvMain, Remember: []string{"env_from"}}})

	recorded, err := w.meta.Load(split)
	require.NoError(t, err)
	assert.Equal(t, project.EnvMain, recorded.EnvStrategy)
	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `env_strategy = "main"`)
}

func TestTheConfigDefaultIsRememberedAsSuchOnceTheTargetIsCreated(t *testing.T) {
	w := withEnvFiles(t, envToml)

	w.apply(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{EnvFrom: provision.EnvFromConfig, Remember: []string{"env_from"}}})

	recorded, err := w.meta.Load(split)
	require.NoError(t, err)
	assert.Equal(t, project.EnvExample, recorded.EnvStrategy)
	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `env_strategy = "config"`)
}

func TestAnApplyReportsTheTargetAsItsOneItem(t *testing.T) {
	cases := []struct {
		name   string
		ctx    func(t *testing.T) context.Context
		status domain.Status
	}{
		{name: "done", ctx: func(t *testing.T) context.Context { return t.Context() }, status: domain.StatusDone},
		{name: "cancelledFirst", ctx: func(t *testing.T) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, status: domain.StatusSkipped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, seed{})
			w.change(t, source, "a.txt", "a\n")
			planned, _ := w.plan(t, moving("a.txt"))

			result := w.usecase.Apply(tc.ctx(t), planned, func(domain.Event) {})

			require.Len(t, result.Items, 1)
			assert.Equal(t, tc.status, result.Items[0].Status)
		})
	}
}
