package env_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADriftedWorktreeGetsItsKeysAndItsPortsBack(t *testing.T) {
	w := newWorld(acmeSeed())

	outcome := w.run(t, naming(billing), domain.RunOptions{})

	require.NotNil(t, outcome.Result)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
	assert.Equal(t, []string{apiEnv, webEnv}, outcome.Result.Items[0].Detail.Written)
	assert.Equal(t, "PORT=4020\nOLD=1\nJWT_SECRET=s3cret\n", w.file(t, billing, apiEnv))
	assert.Equal(t, "PORT=3020\nAPI_URL=http://localhost:4020\n", w.file(t, billing, webEnv))
}

func TestAConflictAndAnOrphanAreLeftUnlessAsked(t *testing.T) {
	s := acmeSeed()
	s.Files["/repo/.trees/"+login+"/"+apiEnv] = "PORT=4010\nJWT_SECRET=mine\nOLD=1\n"
	cases := map[string]struct {
		req  env.Request
		want string
	}{
		"left":      {req: naming(login), want: "PORT=4010\nJWT_SECRET=mine\nOLD=1\n"},
		"overwrite": {req: env.Request{Worktrees: []string{login}, Overwrite: true}, want: "PORT=4010\nJWT_SECRET=s3cret\nOLD=1\n"},
		"prune":     {req: env.Request{Worktrees: []string{login}, Prune: true}, want: "PORT=4010\nJWT_SECRET=mine\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(s)

			w.run(t, c.req, domain.RunOptions{})

			assert.Equal(t, c.want, w.file(t, login, apiEnv))
		})
	}
}

func TestAReconciledWorktreeHasNothingLeftToReconcile(t *testing.T) {
	// legacy: `wtm env --check` still reported drift after `wtm env --yes`, counting the orphans and conflicts an unattended run leaves
	w := newWorld(acmeSeed())
	w.run(t, naming(billing), domain.RunOptions{})

	again := w.run(t, naming(billing), domain.RunOptions{DryRun: true})

	assert.True(t, again.Plan.Empty)
	assert.Equal(t, 0, optionOf(w.questions(t, env.Request{}).Worktrees.Options, billing).OutOfStep)
}

func TestTheDefaultPickIsTheCallersAndAPersonIsOfferedTheWorktreesOutOfStepToo(t *testing.T) {
	w := newWorld(acmeSeed())

	q := w.questions(t, env.Request{Caller: domain.Caller{Cwd: "/repo/.trees/" + login + "/apps"}})

	assert.Equal(t, domain.OriginDefault, q.Worktrees.Picked.Origin)
	assert.Equal(t, []string{login}, q.Worktrees.Picked.Value)
	assert.Equal(t, []string{billing, fresh, login}, q.Worktrees.Prechecked)
	assert.Equal(t, 2, optionOf(q.Worktrees.Options, billing).OutOfStep)
	assert.Equal(t, 0, optionOf(q.Worktrees.Options, login).OutOfStep)
	assert.Equal(t, project.Verbatim, optionOf(q.Worktrees.Options, verbatim).Isolation)
}

func TestOverwriteAndPruneAreAskedOnlyWhenTheWorktreesAskedHaveSome(t *testing.T) {
	w := newWorld(acmeSeed())

	inStep := w.questions(t, naming(login))
	drifted := w.questions(t, naming(billing))

	assert.Equal(t, domain.SkipNoKeyConflict, inStep.Overwrite.Skip)
	assert.Equal(t, domain.SkipNoKeyOrphan, inStep.Prune.Skip)
	assert.Equal(t, domain.SkipNoKeyConflict, drifted.Overwrite.Skip)
	assert.Empty(t, drifted.Prune.Skip)
}

func TestValuesAreWithheldUnlessShown(t *testing.T) {
	// legacy: the wizard printed the secrets the report withheld
	w := newWorld(acmeSeed())

	hidden := w.run(t, naming(billing), domain.RunOptions{DryRun: true})
	shown := w.run(t, env.Request{Worktrees: []string{billing}, ShowValues: true}, domain.RunOptions{DryRun: true})

	hiddenKey := reconcileOf(t, hidden.Plan, billing).Files[0].Keys[0]
	shownKey := reconcileOf(t, shown.Plan, billing).Files[0].Keys[0]
	assert.Nil(t, hiddenKey.Source)
	assert.Empty(t, reconcileOf(t, hidden.Plan, billing).Ports.Entries[1].CurrentValue)
	require.NotNil(t, shownKey.Source)
	assert.Equal(t, "s3cret", *shownKey.Source)
	assert.Equal(t, "3010", reconcileOf(t, shown.Plan, billing).Ports.Entries[1].CurrentValue)
}

func TestAnUnnumberedWorktreeIsNumberedThenItsPortsSettled(t *testing.T) {
	w := newWorld(acmeSeed())

	planned := w.run(t, naming(fresh), domain.RunOptions{DryRun: true})
	outcome := w.run(t, naming(fresh), domain.RunOptions{})

	assert.False(t, reconcileOf(t, planned.Plan, fresh).Numbered)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
	meta, _ := w.store.Meta(fresh)
	assert.Equal(t, 4, meta.Ordinal)
	assert.Equal(t, "PORT=3040\nAPI_URL=http://localhost:4040\n", w.file(t, fresh, webEnv))
}

func TestAVerbatimWorktreeKeepsTheValuesOfItsSource(t *testing.T) {
	w := newWorld(acmeSeed())

	outcome := w.run(t, naming(verbatim), domain.RunOptions{})

	assert.Equal(t, domain.ReasonUnchanged, outcome.Result.Items[0].Reason)
	assert.Equal(t, "PORT=3000\nAPI_URL=http://localhost:4000\n", w.file(t, verbatim, webEnv))
}

func TestAMissingFileIsRebuiltFromItsSource(t *testing.T) {
	s := acmeSeed()
	delete(s.Files, "/repo/.trees/"+login+"/"+apiEnv)
	w := newWorld(s)

	outcome := w.run(t, naming(login), domain.RunOptions{})

	assert.Equal(t, []string{apiEnv}, outcome.Result.Items[0].Detail.Written)
	assert.Equal(t, "PORT=4010\nJWT_SECRET=s3cret\n", w.file(t, login, apiEnv))
}

func TestAJobUpThatReadsAFileWrittenIsToRestart(t *testing.T) {
	s := acmeSeed()
	s.Jobs = []run.Job{{Name: "web", WorkDir: "/repo/.trees/" + billing, Up: true}, {Name: "api", WorkDir: "/repo/.trees/" + login, Up: true}}
	w := newWorld(s)

	outcome := w.run(t, naming(billing, login), domain.RunOptions{})

	assert.Equal(t, []string{"web"}, reconcileOf(t, outcome.Plan, billing).Restart)
	assert.Equal(t, []string{env.CodeRestart}, codes(outcome.Result.Items[0].Warnings))
	assert.Empty(t, outcome.Result.Items[1].Warnings)
	require.NotNil(t, outcome.Result.Items[0].Warnings[0].FollowUp)
	assert.Equal(t, domain.CommandRunStart, outcome.Result.Items[0].Warnings[0].FollowUp.Command)
}

func TestTheMainCheckoutSpelledWithPortsKeepsItsPorts(t *testing.T) {
	s := acmeSeed()
	s.Run.Addressing = ""
	w := newWorld(s)
	w.usecase.Provision.PublicPort = func(_ context.Context, _ int) int { return 11080 }

	outcome := w.run(t, naming("main"), domain.RunOptions{DryRun: true})

	assert.True(t, outcome.Plan.Empty)
}

func TestARequestTheWorktreesCannotTakeIsRefused(t *testing.T) {
	s := acmeSeed()
	s.Git.Worktrees = append(s.Git.Worktrees, worktree.Worktree{Path: "/repo/.trees/detached", State: worktree.Detached{}})
	w := newWorld(s)

	cases := map[string]struct {
		req  env.Request
		want []string
	}{
		"all_and_names": {req: env.Request{Worktrees: []string{billing}, All: true}, want: []string{env.CodeAllAndNames}},
		"named":         {req: naming("/repo/.trees/detached", "nope", billing, billing), want: []string{env.CodeDetached, domain.CodeNotFound, domain.CodeDistinct}},
		"strategy":      {req: env.Request{Worktrees: []string{billing}, Strategy: "copy"}, want: []string{domain.CodeOneOf}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := w.usecase.Run(t.Context(), c.req, domain.RunOptions{})

			failure, ok := errors.AsType[*domain.Error](err)
			require.True(t, ok, err)
			var found []string
			for _, field := range failure.Fields {
				found = append(found, field.Code)
			}
			assert.Equal(t, c.want, found)
		})
	}
}

func TestAnUnreadableRunTomlLeavesThePortsAndSaysWhy(t *testing.T) {
	s := acmeSeed()
	s.RunErr = errors.New("run.toml: bad")
	w := newWorld(s)

	outcome := w.run(t, naming(billing), domain.RunOptions{})

	assert.Contains(t, codes(outcome.Warnings), env.CodeUnsettled)
	assert.Equal(t, "PORT=3010\nAPI_URL=http://localhost:4020\n", w.file(t, billing, webEnv))
}

func TestAWorktreeYetToChooseItsIsolationKeepsItsPortsUntilItDoes(t *testing.T) {
	s := acmeSeed()
	s.Meta[billing] = worktree.Meta{SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 2}
	w := newWorld(s)

	left := w.run(t, naming(billing), domain.RunOptions{})
	q := w.questions(t, naming(billing))
	isolated := w.run(t, env.Request{Worktrees: []string{billing}, Isolation: project.Isolated}, domain.RunOptions{})

	assert.Contains(t, codes(left.Warnings), env.CodePending)
	assert.Empty(t, q.Isolation.Skip)
	assert.Equal(t, project.Isolated, reconcileOf(t, isolated.Plan, billing).Records)
	assert.Equal(t, "PORT=3020\nAPI_URL=http://localhost:4020\n", w.file(t, billing, webEnv))
	meta, _ := w.store.Meta(billing)
	assert.Equal(t, project.Isolated, meta.Isolation)
}

func TestSwitchingToVerbatimPutsBackTheSourceValues(t *testing.T) {
	w := newWorld(acmeSeed())

	w.run(t, env.Request{Worktrees: []string{login}, Isolation: project.Verbatim}, domain.RunOptions{})

	assert.Equal(t, "PORT=3000\nAPI_URL=http://localhost:4000\n", w.file(t, login, webEnv))
	meta, _ := w.store.Meta(login)
	assert.Equal(t, project.Verbatim, meta.Isolation)
}

func TestTheMainCheckoutIsAlwaysIsolated(t *testing.T) {
	w := newWorld(acmeSeed())

	_, err := w.usecase.Run(t.Context(), env.Request{Worktrees: []string{"main"}, Isolation: project.Verbatim}, domain.RunOptions{})

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, env.CodeMainIsolated, failure.Fields[0].Code)
}

func TestRecordingAnIsolationAloneIsAChange(t *testing.T) {
	// the proof: a worktree whose files were already in line said "unchanged" while its isolation was recorded
	s := acmeSeed()
	s.Meta[login] = worktree.Meta{SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 1}
	w := newWorld(s)

	outcome := w.run(t, env.Request{Worktrees: []string{login}, Isolation: project.Isolated}, domain.RunOptions{})

	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
	assert.Empty(t, outcome.Result.Items[0].Reason)
}

func TestAPortWtmFindsNothingToMoveInIsLeftAloneAndSaid(t *testing.T) {
	// the proof: a port set by hand was "in step", where legacy warned it was left alone
	s := acmeSeed()
	s.Files["/repo/.trees/"+login+"/"+webEnv] = "PORT=9999\nAPI_URL=http://localhost:4010\n"
	w := newWorld(s)

	outcome := w.run(t, naming(login), domain.RunOptions{DryRun: true})

	require.Contains(t, codes(outcome.Warnings), env.CodeLeftAlone)
	warning := outcome.Warnings[slices.IndexFunc(outcome.Warnings, func(w domain.Warning) bool { return w.Code == env.CodeLeftAlone })]
	require.Len(t, warning.Ports, 1)
	assert.Equal(t, "PORT", warning.Ports[0].Key)
}

func TestAnUnattendedRunWithNoneNamedWritesTheCallersWorktreeAlone(t *testing.T) {
	// the review: run from main, it wrote into every worktree out of step, which nobody named
	w := newWorld(acmeSeed())

	outcome := w.run(t, env.Request{Caller: domain.Caller{Cwd: mainPath}}, domain.RunOptions{})

	require.Len(t, outcome.Result.Items, 1)
	assert.Equal(t, "main", outcome.Result.Items[0].Subject)
	assert.Equal(t, "PORT=3010\nAPI_URL=http://localhost:4020\n", w.file(t, billing, webEnv))
}

func TestAnUnreadableWorktreeIsLeftAndRecordsNoIsolation(t *testing.T) {
	// the review: its isolation was recorded with nothing written, and it was said in step
	w := newWorld(acmeSeed())
	w.usecase.EnvFiles = failingIn{Files: w.store, dir: "/repo/.trees/" + login + "/", read: errors.New("permission denied")}

	outcome := w.run(t, env.Request{Worktrees: []string{login}, Isolation: project.Verbatim}, domain.RunOptions{})

	assert.Equal(t, domain.ReasonRuledOut, outcome.Result.Items[0].Reason)
	assert.Contains(t, codes(outcome.Warnings), env.CodeUnreadable)
	assert.Equal(t, "left", outcome.Plan.Recap[len(outcome.Plan.Recap)-1].Key)
	meta, _ := w.store.Meta(login)
	assert.Equal(t, project.Isolated, meta.Isolation)
}

func TestTheValuesASwitchToVerbatimPutsBackAreNoConflictToAskAbout(t *testing.T) {
	// the review: the overwrite question listed the ports a switch to verbatim always restores
	w := newWorld(acmeSeed())

	q := w.questions(t, env.Request{Worktrees: []string{login}, Isolation: project.Verbatim})

	assert.Equal(t, domain.SkipNoKeyConflict, q.Overwrite.Skip)
	assert.Empty(t, q.Conflicts)
}

func TestAnIsolationChosenForAPendingWorktreeLeavesTheOthersTheirs(t *testing.T) {
	// the review: a verbatim worktree picked beside a pending one became isolated
	s := acmeSeed()
	s.Meta[billing] = worktree.Meta{SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 2}
	w := newWorld(s)

	outcome := w.run(t, env.Request{Worktrees: []string{billing, verbatim}, Isolation: project.Isolated}, domain.RunOptions{DryRun: true})

	assert.Equal(t, project.Isolated, reconcileOf(t, outcome.Plan, billing).Records)
	assert.Equal(t, project.Verbatim, reconcileOf(t, outcome.Plan, verbatim).Isolation)
	assert.Empty(t, reconcileOf(t, outcome.Plan, verbatim).Records)
}

func TestOneWorktreeFailingStopsNoneOfTheOthers(t *testing.T) {
	s := acmeSeed()
	s.Files["/repo/.trees/"+login+"/"+webEnv] = "PORT=3000\nAPI_URL=http://localhost:4010\n"
	w := newWorld(s)
	w.usecase.EnvFiles = failingIn{Files: w.store, dir: "/repo/.trees/" + login + "/", edit: errors.New("disk full")}

	outcome := w.run(t, naming(login, billing), domain.RunOptions{})

	assert.Equal(t, domain.StatusFailed, outcome.Result.Items[0].Status)
	assert.Equal(t, env.CodeFailed, outcome.Result.Items[0].Err.Code)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[1].Status)
	assert.Equal(t, "PORT=3020\nAPI_URL=http://localhost:4020\n", w.file(t, billing, webEnv))
}

func TestACancelledApplySkipsTheWorktreesNotStarted(t *testing.T) {
	w := newWorld(acmeSeed())
	planned, _, err := w.usecase.Plan(t.Context(), naming(billing))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := w.usecase.Apply(ctx, planned, func(domain.Event) {})

	assert.Equal(t, domain.ReasonInterrupted, result.Items[0].Reason)
	assert.Equal(t, "PORT=3010\nAPI_URL=http://localhost:4020\n", w.file(t, billing, webEnv))
}

func TestTheFingerprintCoversWhatIsWrittenNotTheJobsUpNorTheValuesShown(t *testing.T) {
	s := acmeSeed()
	plain := newWorld(s).run(t, naming(billing), domain.RunOptions{DryRun: true})
	shown := newWorld(s).run(t, env.Request{Worktrees: []string{billing}, ShowValues: true}, domain.RunOptions{DryRun: true})
	s.Jobs = []run.Job{{Name: "web", WorkDir: "/repo/.trees/" + billing, Up: true}}
	running := newWorld(s).run(t, naming(billing), domain.RunOptions{DryRun: true})
	s.Files["/repo/.trees/"+billing+"/"+webEnv] = "PORT=3011\nAPI_URL=http://localhost:4020\n"
	edited := newWorld(s).run(t, naming(billing), domain.RunOptions{DryRun: true})

	assert.Equal(t, plain.Fingerprint, shown.Fingerprint)
	assert.Equal(t, plain.Fingerprint, running.Fingerprint)
	assert.NotEqual(t, plain.Fingerprint, edited.Fingerprint)
}
