package runstart_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStartPlansTheOneJobInItsWorktree(t *testing.T) {
	w := newWorld(t)

	planned, warnings := w.plan(t, starting("api", featA))

	assert.Empty(t, warnings)
	require.Len(t, planned.Worktrees, 1)
	assert.Equal(t, []runjobs.JobPlan{{Name: "api", Kind: project.Service, Action: runjobs.ActionStart, Step: 1, Ports: map[string]int{"PORT": 4010}}}, planned.Worktrees[0].Jobs)
	assert.Equal(t, domain.WorktreeLocks([]string{featA}), planned.Locks())
}

func TestAStartStartsTheJobAloneWithNoProfileAndFollowsWithItsStop(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, starting("api", featA))

	require.Len(t, w.daemon.started, 1)
	spec := w.daemon.specOf(t, w.at(featA), "api")
	assert.Empty(t, spec.Profile)
	assert.False(t, spec.Restart)
	assert.Equal(t, map[string]runjobs.JobStatus{"api": runjobs.JobStarted}, statusesOf(result.Items[0]))
	assert.Equal(t, []string{runstart.CodeLogs, runstart.CodeDown}, followUpCodes(result.FollowUps))
}

func TestAServiceUpIsLeftAsItIsAnEmptyPlan(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)

	planned, _ := w.plan(t, starting("api", featA))

	assert.Equal(t, runjobs.ActionUp, planned.Worktrees[0].Jobs[0].Action)
	assert.True(t, planned.Empty)
}

// Q2: --restart stops the job, then starts it again, its log continued behind the run mark.
func TestARestartStopsTheJobThenStartsItAgainItsLogAppended(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)
	req := starting("api", featA)
	req.Restart = true

	planned, _ := w.plan(t, req)
	result, report := w.apply(t, req)

	assert.Equal(t, runjobs.ActionRestart, planned.Worktrees[0].Jobs[0].Action)
	assert.False(t, planned.Empty)
	assert.Equal(t, []string{w.at(featA) + " api"}, w.daemon.stopped)
	assert.True(t, w.daemon.specOf(t, w.at(featA), "api").Restart)
	assert.Equal(t, map[string]runjobs.JobStatus{"api": runjobs.JobStarted}, statusesOf(result.Items[0]))
	assert.Len(t, eventsOf[domain.PhaseStarted](report.All()), 1, "the stop and the start are one phase of the job")
}

func TestARestartOfAJobNotUpStartsIt(t *testing.T) {
	w := newWorld(t)
	req := starting("api", featA)
	req.Restart = true

	planned, _ := w.plan(t, req)

	assert.Equal(t, runjobs.ActionStart, planned.Worktrees[0].Jobs[0].Action)
}

func TestARestartWhoseStopFailsStartsNothing(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)
	w.daemon.stopErr = &run.Error{Code: run.CodeStopFailed, Message: "the stop command failed"}
	req := starting("api", featA)
	req.Restart = true

	result, _ := w.apply(t, req)

	assert.Empty(t, w.daemon.started)
	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Equal(t, runstart.CodeJobFailed, result.Items[0].Err.Code)
	assert.Contains(t, result.Items[0].Err.Message, "api runs on")
	assert.Equal(t, map[string]runjobs.JobStatus{"api": runjobs.JobAlreadyRunning}, statusesOf(result.Items[0]), "a stop that failed left it running")
}

func TestARequestsProblemsAreRefusedAsFieldErrors(t *testing.T) {
	w := newWorld(t)
	// db up for feat/b, which feat/a does not hold
	w.daemon.listed = append(w.daemon.listed,
		run.Job{Name: "db", WorkDir: w.main, SharedDir: w.main, Up: true, State: run.Running, Hold: run.HoldOwn},
		run.Job{Name: "db", WorkDir: w.at(featB), SharedDir: w.main, Up: true, Hold: run.HoldClaim},
	)
	restartShared := starting("db", featA)
	restartShared.Restart = true
	cases := map[string]struct {
		req  runstart.Request
		path string
		code string
	}{
		"no_job":         {req: starting("", featA), path: runstart.PathJob, code: domain.CodeRequired},
		"unknown_job":    {req: starting("ghost", featA), path: runstart.PathJob, code: domain.CodeNotFound},
		"restart_shared": {req: restartShared, path: runstart.PathRestart, code: runstart.CodeRestartShared},
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

// B10: legacy asked whether to stop the other worktrees' jobs for a task.
func TestATaskIsNeverAskedToStopTheOthersFirst(t *testing.T) {
	w := newWorld(t)
	w.up("api", featB, nil)

	task := w.questions(t, starting("migrate", featA))
	service := w.questions(t, starting("web", featA))

	assert.Equal(t, domain.SkipTask, task.Exclusive.Skip)
	assert.Empty(t, service.Exclusive.Skip)
}

// B8: legacy's picker offered the jobs already running, which then ended on "already running".
func TestTheJobsOfferedSayWhetherEachIsUpInTheWorktree(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)

	questions := w.questions(t, starting("", featA))

	require.NotEmpty(t, questions.Jobs)
	assert.Equal(t, domain.Option{Value: "api", Kind: project.Service, RunningIn: []string{featA}}, questions.Jobs[0])
	assert.Equal(t, domain.Option{Value: "web", Kind: project.Service}, questions.Jobs[1])
}

// B11: legacy's foreign-data refusal spelled `wtm env <branch>`; the refusal is followed by the start forced.
func TestAJobChangingForeignDataIsRefusedFollowedByTheStartForced(t *testing.T) {
	w := newWorld(t)

	refused := w.refused(t, starting("migrate", verbatim))

	assert.Equal(t, runstart.CodeUnsafe, refused.Code)
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, runstart.CodeForce, refused.FollowUp.Code)
	assert.Contains(t, refused.FollowUp.Answers, domain.Answered{Path: domain.PathWorktree, Values: []string{verbatim}})
}

// B5: legacy's failing task said "Profile aborted" and offered to stop what did start when nothing did.
func TestATaskThatFailsOffersNoStop(t *testing.T) {
	w := newWorld(t)
	w.daemon.refuse = map[string]error{key(w.at(featA), "migrate"): &run.Error{Code: run.CodeTaskFailed, Message: "exit 3", ExitCode: new(3)}}

	result, _ := w.apply(t, starting("migrate", featA))

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Empty(t, result.FollowUps)
}

// B12: legacy started the daemon before asking anything.
func TestTheQuestionsNeverStartTheDaemon(t *testing.T) {
	w := newWorld(t)
	w.daemon.ensureErr = &run.Error{Code: run.CodeStartFailed, Message: "would start"}

	questions := w.questions(t, starting("api", featA))

	assert.Empty(t, questions.Errors)
}

func TestTheCallersWorktreeIsStartedInWhenNoneIsNamed(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, runstart.Request{Job: "api"}.From(domain.Caller{Cwd: w.at(featA)}))

	assert.Equal(t, featA, planned.Worktrees[0].Worktree)
}

func TestARequestRoundTripsThroughItsAnswers(t *testing.T) {
	yes := true
	req := runstart.Request{Worktree: featA, Job: "api", Restart: true, Exclusive: &yes, Parallel: true, Force: true, NoProbe: true, Detach: true}

	answered := runstart.Request{}
	for _, field := range (runstart.Questions{}).Form().Fields {
		var err error
		answered, err = answered.Answer(field.Spec().Path, req.Values(field.Spec().Path))
		require.NoError(t, err)
	}

	assert.Equal(t, req, answered)
}

// Review: a worktree numbered by the start keeps its restart: Planned, which numbering runs again, knows of none.
func TestARestartInAWorktreeTheStartNumbersStillStopsTheJobFirst(t *testing.T) {
	w := newWorld(t)
	w.up("api", fresh, nil)
	req := starting("api", fresh)
	req.Restart = true

	w.apply(t, req)

	assert.Equal(t, []string{w.at(fresh) + " api"}, w.daemon.stopped)
	assert.True(t, w.daemon.specOf(t, w.at(fresh), "api").Restart)
}

// Review: a restart keeps the profile the job was started under, so the start again of its worktree still names it.
func TestARestartKeepsTheProfileTheJobWasStartedUnder(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)
	w.daemon.listed[0].Profile = "alone"
	req := starting("api", featA)
	req.Restart = true

	w.apply(t, req)

	assert.Equal(t, "alone", w.daemon.specOf(t, w.at(featA), "api").Profile)
}

// Review: a restart cut short once its job stopped is cancelled, the job not started again, never said crashed.
func TestARestartCutShortAfterItsStopIsCancelledNotCrashed(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)
	req := starting("api", featA)
	req.Restart = true
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	w.daemon.onStop = cancel

	result, _ := w.applyIn(ctx, t, req)

	assert.Empty(t, w.daemon.started)
	assert.Equal(t, domain.StatusCancelled, result.Items[0].Status)
	assert.Equal(t, map[string]runjobs.JobStatus{"api": runjobs.JobStopped}, statusesOf(result.Items[0]))
	require.Len(t, result.FollowUps, 1)
	assert.Equal(t, runstart.CodeStart, result.FollowUps[0].Code)
	assert.Contains(t, result.FollowUps[0].Answers, domain.Answered{Path: runstart.PathJob, Values: []string{"api"}})
}

// Review: a restart cut short during its start, the daemon listing the job stopped by the restart's own stop, is no crash.
func TestARestartCutShortDuringItsStartIsStoppedNotCrashed(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)
	req := starting("api", featA)
	req.Restart = true
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	w.daemon.onStart = func(run.Spec) { cancel() }
	w.daemon.refuse = map[string]error{key(w.at(featA), "api"): context.Canceled}

	result, _ := w.applyIn(ctx, t, req)

	assert.Equal(t, domain.StatusCancelled, result.Items[0].Status)
	assert.Equal(t, map[string]runjobs.JobStatus{"api": runjobs.JobStopped}, statusesOf(result.Items[0]))
}

// Review: --restart of a job that crashed starts it with its log continued: its output before the crash is kept.
func TestARestartOfACrashedJobKeepsItsLog(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: "api", WorkDir: w.at(featA), State: run.Crashed, Hold: run.HoldOwn})
	req := starting("api", featA)
	req.Restart = true

	planned, _ := w.plan(t, req)
	w.apply(t, req)

	assert.Equal(t, runjobs.ActionStart, planned.Worktrees[0].Jobs[0].Action)
	assert.Empty(t, w.daemon.stopped)
	assert.True(t, w.daemon.specOf(t, w.at(featA), "api").Restart)
}

// Review: a start that cannot reach the daemon lists the drops owed it would pay unpaid, as run up does.
func TestAStartThatCannotReachTheDaemonListsTheDropsOwedUnpaid(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.stateDir, "pending-removals.toml"), "[[pending]]\n  job = \"db\"\n  worktree = \"feat-old\"\n  ordinal = 4\n")
	w.daemon.listed = []run.Job{{Name: "db", WorkDir: w.main, Up: true, Hold: run.HoldOwn, SharedDir: w.main}}
	w.daemon.ensureErr = &run.Error{Code: run.CodeStartFailed, Message: "no daemon"}

	result, _ := w.apply(t, starting("db", featA))

	require.Len(t, result.Items, 2)
	assert.Equal(t, "app_feat-old", result.Items[1].Subject)
	assert.Equal(t, domain.StatusSkipped, result.Items[1].Status)
	assert.Empty(t, *w.drops)
}

// Review: a stop whose answer was lost, the job stopped all the same, is a failure, never said running on with its address and a stop to follow.
func TestARestartWhoseStopAnswerWasLostIsAFailureNotRunningOn(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)
	w.daemon.stopErr, w.daemon.stopLost = errors.New("run daemon: connection reset"), true
	req := starting("api", featA)
	req.Restart = true

	result, _ := w.apply(t, req)

	assert.Equal(t, map[string]runjobs.JobStatus{"api": runjobs.JobFailed}, statusesOf(result.Items[0]))
	assert.Empty(t, result.Items[0].Detail.Reached())
	assert.Empty(t, result.FollowUps)
}

func TestARestartOfASharedServiceItsWorktreeHoldsRestartsItForEveryHolder(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = append(w.daemon.listed,
		run.Job{Name: "db", WorkDir: w.main, SharedDir: w.main, MainHolds: true, Up: true, State: run.Running, Hold: run.HoldOwn},
		run.Job{Name: "db", WorkDir: w.at(featA), SharedDir: w.main, Up: true, Hold: run.HoldClaim},
		run.Job{Name: "db", WorkDir: w.at(featB), SharedDir: w.main, Up: true, Hold: run.HoldClaim},
	)
	req := starting("db", featA)
	req.Restart = true

	planned, _ := w.plan(t, req)
	result, _ := w.apply(t, req)

	assert.Equal(t, runjobs.ActionRestart, planned.Worktrees[0].Jobs[0].Action)
	assert.Equal(t, []string{"main", featA, featB}, planned.HeldBy, "every worktree holding it stops for a moment")
	assert.Empty(t, w.daemon.stopped, "the daemon restarts it in place: a stop would let go of feat/a's hold")
	assert.True(t, w.daemon.specOf(t, w.at(featA), "db").Restart)
	assert.Equal(t, map[string]runjobs.JobStatus{"db": runjobs.JobStarted}, statusesOf(result.Items[0]))
}

func TestARestartOfASharedServiceTheMainCheckoutRunsWithoutHoldingItIsRefused(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = append(w.daemon.listed,
		run.Job{Name: "db", WorkDir: w.main, SharedDir: w.main, Up: true, State: run.Running, Hold: run.HoldOwn},
		run.Job{Name: "db", WorkDir: w.at(featA), SharedDir: w.main, Up: true, Hold: run.HoldClaim},
	)
	req := starting("db", "main")
	req.Restart = true

	refused := w.refused(t, req)

	require.Len(t, refused.Fields, 1)
	assert.Equal(t, runstart.CodeRestartShared, refused.Fields[0].Code)
}

// Found driving the web: the Undo of main's release was planned up (already running), and main never held it again.
func TestTheMainCheckoutRunningASharedServiceForTheOthersJoinsIt(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = append(w.daemon.listed,
		run.Job{Name: "db", WorkDir: w.main, SharedDir: w.main, Up: true, State: run.Running, Hold: run.HoldOwn},
		run.Job{Name: "db", WorkDir: w.at(featA), SharedDir: w.main, Up: true, Hold: run.HoldClaim},
	)

	planned, _ := w.plan(t, starting("db", "main"))
	result, _ := w.apply(t, starting("db", "main"))

	assert.Equal(t, runjobs.ActionJoin, planned.Worktrees[0].Jobs[0].Action)
	assert.False(t, planned.Empty)
	assert.Equal(t, w.main, w.daemon.specOf(t, w.main, "db").WorkDir, "the daemon is asked: it takes main's own hold")
	assert.Equal(t, map[string]runjobs.JobStatus{"db": runjobs.JobJoined}, statusesOf(result.Items[0]))
}

func TestARestartOfASharedServiceUpNowhereStartsIt(t *testing.T) {
	w := newWorld(t)
	req := starting("db", featA)
	req.Restart = true

	planned, _ := w.plan(t, req)

	assert.Equal(t, runjobs.ActionJoin, planned.Worktrees[0].Jobs[0].Action, "joined, as a start of it")
	assert.Empty(t, planned.HeldBy)
}

func TestARestartFromAMainCheckoutADaemonOfAnotherBuildCannotTellHoldsIsTakenAsHeld(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = append(w.daemon.listed,
		run.Job{Name: "db", WorkDir: w.main, SharedDir: w.main, MainHoldsUnknown: true, Up: true, State: run.Running, Hold: run.HoldOwn},
		run.Job{Name: "db", WorkDir: w.at(featA), SharedDir: w.main, Up: true, Hold: run.HoldClaim},
	)
	req := starting("db", "main")
	req.Restart = true

	planned, _ := w.plan(t, req)

	assert.Equal(t, runjobs.ActionRestart, planned.Worktrees[0].Jobs[0].Action)
	assert.Equal(t, []string{"main", featA}, planned.HeldBy)
}
