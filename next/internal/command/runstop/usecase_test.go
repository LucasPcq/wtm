package runstop_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStopPlansTheJobsFateInEachWorktreeAndNotRunningWhereItIsNot(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	w.up("web", featA, "dev")

	planned, warnings := w.plan(t, w.stopping("api", featA, featB))

	assert.Empty(t, warnings)
	require.Len(t, planned.Worktrees, 2)
	assert.Equal(t, []runjobs.StopJob{{Name: "api", Kind: project.Service, Hold: run.HoldOwn, Fate: run.FateStop}}, planned.Worktrees[0].Jobs)
	assert.Empty(t, planned.Worktrees[1].Jobs)
	assert.Equal(t, []string{"api", "api (not running)"}, []string{planned.Recap[2].Values[0].Text, planned.Recap[3].Values[0].Text})
	assert.Equal(t, domain.WorktreeLocks([]string{featA, featB}), planned.Locks())
}

func TestAStopStopsTheJobAloneAndFollowsWithItsStartAgain(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	w.up("web", featA, "dev")

	result, report := w.apply(t, w.stopping("api", featA, featB))

	assert.Equal(t, []string{key(w.at(featA), "api")}, w.daemon.stopped)
	assert.Equal(t, map[string]runjobs.StopStatus{"api": runjobs.StopStopped}, statuses(result.Items[0]))
	assert.Equal(t, map[string]runjobs.StopStatus{"api": runjobs.StopNotRunning}, statuses(result.Items[1]))
	assert.Equal(t, domain.ReasonUnchanged, result.Items[1].Reason)
	assert.Contains(t, report.All(), domain.PhaseStarted{Subject: featA, Phase: domain.PhaseJob, Job: "api"})
	assert.Equal(t, []domain.FollowUp{{Command: domain.CommandRunStart, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA}}, {Path: "job", Values: []string{"api"}}}, Code: runstop.CodeStart, Message: "start it again"}}, result.FollowUps)
}

func TestASharedServiceIsReleasedForTheWorktreesKeepingIt(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA, featB)
	w.daemon.outcomes = map[string]run.Outcome{key(w.at(featA), "db"): run.OutcomeReleased}

	planned, _ := w.plan(t, w.stopping("db", featA))
	result, _ := w.apply(t, w.stopping("db", featA))

	assert.Equal(t, []runjobs.StopJob{{Name: "db", Kind: project.Service, Hold: run.HoldClaim, Fate: run.FateRelease, KeptBy: []string{featB}, SharedIn: "main"}}, planned.Worktrees[0].Jobs)
	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopReleased}, statuses(result.Items[0]))
	assert.Len(t, result.FollowUps, 1)
}

func TestAStopThatFailsIsThatWorktreesAndTheOthersGoOn(t *testing.T) {
	w := newWorld(t)
	w.up("cache", featA, "full")
	w.up("cache", featB, "full")
	w.daemon.refuse = map[string]error{key(w.at(featA), "cache"): errStack}

	result, _ := w.apply(t, w.stopping("cache", featA, featB))

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Equal(t, runstop.Name+runjobs.StopFailed, result.Items[0].Err.Code)
	assert.Equal(t, map[string]runjobs.StopStatus{"cache": runjobs.StopStopped}, statuses(result.Items[1]))
	assert.Equal(t, featB, result.FollowUps[0].Answers[0].Values[0])
}

func TestARequestsProblemsAreRefusedAsFieldErrors(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	cases := map[string]struct {
		req  runstop.Request
		path string
		code string
	}{
		"no_job":      {req: w.stopping("", featA), path: runstop.PathJob, code: domain.CodeRequired},
		"unknown_job": {req: w.stopping("ghost", featA), path: runstop.PathJob, code: domain.CodeNotFound},
		"unknown":     {req: w.stopping("api", "feat/ghost"), path: "worktree[0]", code: domain.CodeNotFound},
		"twice":       {req: w.stopping("api", featA, featA), path: "worktree[1]", code: domain.CodeDistinct},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			refused := w.refused(t, c.req)

			require.Len(t, refused.Fields, 1)
			assert.Equal(t, c.path, refused.Fields[0].Path)
			assert.Equal(t, c.code, refused.Fields[0].Code)
		})
	}
}

func TestTheJobsOfferedAreRunTomlsThenThoseTheDaemonRunsThereEachSayingWhereItIsUp(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	w.up("legacy-worker", featA, "")

	questions, err := w.usecase.Questions(t.Context(), w.stopping("", featA, featB))

	require.NoError(t, err)
	assert.Equal(t, []domain.Option{
		{Value: "api", Kind: project.Service, RunningIn: []string{featA}},
		{Value: "web", Kind: project.Service},
		{Value: "cache", Kind: project.Service},
		{Value: "db", Kind: project.Service},
		{Value: "legacy-worker", Kind: project.Service, RunningIn: []string{featA}},
	}, questions.Jobs)
}

func TestAJobRunTomlDoesNotDeclareStopsWhereTheDaemonRunsIt(t *testing.T) {
	w := newWorld(t)
	w.up("legacy-worker", featA, "")

	result, _ := w.apply(t, w.stopping("legacy-worker", featA))

	assert.Equal(t, map[string]runjobs.StopStatus{"legacy-worker": runjobs.StopStopped}, statuses(result.Items[0]))
}

func TestAnInvalidRunTomlTakesTheJobByItsName(t *testing.T) {
	w := newWorld(t)
	writeFile(t, w.stateDir+"/run.toml", "[[job]\n")
	w.up("api", featA, "dev")

	planned, warnings := w.plan(t, w.stopping("api", featA))

	assert.Len(t, planned.Worktrees[0].Jobs, 1)
	require.Len(t, warnings, 1)
	assert.Equal(t, runstop.CodeRunInvalid, warnings[0].Code)
}

func TestTheCallersWorktreeIsStoppedInWhenNoneIsNamed(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")

	planned, _ := w.plan(t, runstop.Request{Job: "api"}.From(domain.Caller{Cwd: w.at(featA)}))

	require.Len(t, planned.Worktrees, 1)
	assert.Equal(t, featA, planned.Worktrees[0].Worktree)
}

func TestARemovedWorktreesJobStopsWithNoStartAgain(t *testing.T) {
	w := newWorld(t)
	removed := w.root + "/wt-removed"
	w.removedWith("api", removed)

	result, _ := w.apply(t, w.stopping("api", removed))

	assert.Equal(t, map[string]runjobs.StopStatus{"api": runjobs.StopStopped}, statuses(result.Items[0]))
	assert.Empty(t, result.FollowUps)
}

func TestTheQuestionsLeaveTheDaemonAloneThePlanStartsOneToReachItsIndex(t *testing.T) {
	w := newWorld(t)
	w.daemon.unknown = true
	w.up("api", featA, "dev")

	_, err := w.usecase.Questions(t.Context(), w.stopping("api", featA))

	require.NoError(t, err)
	assert.False(t, w.daemon.ensured)
	w.plan(t, w.stopping("api", featA))
	assert.True(t, w.daemon.ensured)
}

func TestADaemonTheClientCannotGoOnWithIsRefusedFollowedByItsRestart(t *testing.T) {
	w := newWorld(t)
	w.daemon.jobsErr = &run.Error{Code: run.CodeSkew, Message: "wtm's daemon speaks protocol 1, this wtm 2"}

	refused := w.refused(t, w.stopping("api", featA))

	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	assert.Equal(t, runstop.CodeDaemonSkew, refused.Code)
	require.NotNil(t, refused.FollowUp)
}

func TestARequestRoundTripsThroughItsAnswers(t *testing.T) {
	req := runstop.Request{Worktrees: []string{featA, featB}, Job: "api"}

	answered := runstop.Request{}
	for _, field := range (runstop.Questions{}).Form().Fields {
		var err error
		answered, err = answered.Answer(field.Spec().Path, req.Values(field.Spec().Path))
		require.NoError(t, err)
	}

	assert.Equal(t, req, answered)
}
