package runup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheDefaultProfileStartsEachJobInOrderUnderItsPhase(t *testing.T) {
	w := newWorld(t)

	result, report := w.apply(t, starting("", featA))

	item := itemOf(t, result.Items, featA)
	require.Equal(t, domain.StatusDone, item.Status, item.Err)
	assert.Equal(t, "dev", item.Detail.Profile)
	assert.Equal(t, map[string]runup.JobStatus{"api": runup.JobStarted, "web": runup.JobStarted}, statusesOf(item))
	var jobs []string
	for _, phase := range eventsOf[domain.PhaseStarted](report.All()) {
		assert.Equal(t, domain.PhaseJob, phase.Phase)
		jobs = append(jobs, phase.Job)
	}
	assert.Equal(t, []string{"api", "web"}, jobs)
	assert.Equal(t, "dev", w.daemon.specOf(t, w.at(featA), "api").Profile, "each job records the profile it was started under")
}

// Two jobs declare PORT: each one reads its own, offset for the worktree (the hooks' environment leaves such a name out).
func TestEachJobReadsItsOwnPortsOffsetForItsWorktree(t *testing.T) {
	w := newWorld(t)

	w.apply(t, starting("", featA))

	assert.Equal(t, "4010", w.daemon.specOf(t, w.at(featA), "api").Env["PORT"])
	assert.Equal(t, "3010", w.daemon.specOf(t, w.at(featA), "web").Env["PORT"])
}

func TestATaskThatFailsStopsItsWorktreeAloneAndIsOfferedAgainWithItsProfile(t *testing.T) {
	w := newWorld(t)
	exit := 3
	w.daemon.refuse = map[string]error{key(w.at(featA), "migrate"): &run.Error{Code: run.CodeTaskFailed, Message: "run daemon: task migrate exited with code 3", ExitCode: &exit}}
	w.daemon.lines = map[string][]string{key(w.at(featA), "migrate"): {"migrating", "no such table"}}

	result, _ := w.apply(t, starting("full", featA, featB))

	failed := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusFailed, failed.Status)
	assert.Equal(t, runup.CodeJobFailed, failed.Err.Code)
	assert.Equal(t, "3", failed.Err.Params["exit_code"])
	assert.Equal(t, map[string]runup.JobStatus{"migrate": runup.JobFailed, "db": runup.JobNotStarted, "api": runup.JobNotStarted, "web": runup.JobNotStarted}, statusesOf(failed))
	assert.Equal(t, []string{"migrating", "no such table"}, failed.Detail.Jobs[0].Lines)
	assert.Equal(t, domain.StatusDone, itemOf(t, result.Items, featB).Status, "one worktree failing stops none of the others")
	assert.Equal(t, []string{runup.CodeRetry}, followUpCodes(result.FollowUps), "run logs and run down are the legacy daemon's until next runs them: neither is offered")
	retry := result.FollowUps[0]
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA}}, {Path: runup.PathProfile, Values: []string{"full"}}}, retry.Answers, "the retry names the worktree and the profile")
}

func TestAServiceThatEndedOnceStartedFailsItsWorktreeCrashed(t *testing.T) {
	w := newWorld(t)
	w.daemon.crash = map[string]bool{key(w.at(featA), "web"): true}

	result, _ := w.apply(t, starting("", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusFailed, item.Status)
	assert.Equal(t, runup.CodeCrashed, item.Err.Code)
	assert.Equal(t, map[string]runup.JobStatus{"api": runup.JobStarted, "web": runup.JobCrashed}, statusesOf(item))
	assert.Equal(t, []string{"boom"}, item.Detail.Jobs[1].Lines)
	assert.Equal(t, 1, *item.Detail.Jobs[1].ExitCode)
}

func TestAProfileAlreadyUpChangesNothingAndSaysSo(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, nil)
	w.up("web", featA, nil)
	w.daemon.refuse = map[string]error{
		key(w.at(featA), "api"): &run.Error{Code: run.CodeJobRunning},
		key(w.at(featA), "web"): &run.Error{Code: run.CodeJobRunning},
	}

	planned, _ := w.plan(t, starting("", featA))
	result, _ := w.apply(t, starting("", featA))

	assert.True(t, planned.Empty)
	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.ReasonUnchanged, item.Reason)
	assert.Equal(t, "http://api.up", item.Detail.Jobs[0].URL, "a job up reports where it answers")
}

func TestASharedServiceIsJoinedFromAWorktreeAndItsNamespaceRecorded(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, starting("full", featA))

	item := itemOf(t, result.Items, featA)
	require.Equal(t, domain.StatusDone, item.Status, item.Err)
	assert.Equal(t, runup.JobJoined, statusesOf(item)["db"])
	assert.Equal(t, "app_feat-a", item.Detail.Jobs[1].Namespace)
	spec := w.daemon.specOf(t, w.at(featA), "db")
	require.NotNil(t, spec.Shared)
	assert.Equal(t, w.main, spec.Shared.WorkDir)
	assert.Equal(t, "5432", spec.Shared.Env["DB_PORT"], "a shared service binds its base port")
	assert.Equal(t, []string{"db"}, w.stored(t, featA).Namespaces)
}

// The main checkout runs a shared service itself: its start names the main checkout all the same, or the daemon refuses it.
func TestASharedServiceStartedFromTheMainCheckoutNamesIt(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, starting("full", "main"))

	require.Equal(t, domain.StatusDone, itemOf(t, result.Items, "main").Status)
	spec := w.daemon.specOf(t, w.main, "db")
	require.NotNil(t, spec.Shared)
	assert.Equal(t, w.main, spec.Shared.WorkDir)
}

func TestAWorktreeWithNoOrdinalIsNumberedBeforeItsJobsStart(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, starting("full", fresh))
	result, _ := w.apply(t, starting("full", fresh))

	assert.False(t, planned.Worktrees[0].Numbered)
	item := itemOf(t, result.Items, fresh)
	require.Equal(t, domain.StatusDone, item.Status, item.Err)
	assert.Equal(t, 3, w.stored(t, fresh).Ordinal)
	assert.Equal(t, "4030", w.daemon.specOf(t, w.at(fresh), "api").Env["PORT"])
	api := item.Detail.Jobs[2]
	require.Len(t, api.Ports, 1, "its ports are the ordinal's once numbered")
	assert.Equal(t, 4030, api.Ports[0].Port)
	assert.True(t, *api.Ports[0].Listening, "and they are checked")
	assert.Equal(t, "app_feat-fresh", item.Detail.Jobs[1].Namespace)
}

func TestADaemonOfAnotherBuildHoldingJobsRefusesTheStartWithItsRestart(t *testing.T) {
	w := newWorld(t)
	w.up("api", featB, nil)
	w.daemon.skewed = &run.Skewed{Skew: run.SkewBuild, Daemon: "0.9"}

	refused := w.refused(t, starting("", featA))

	assert.Equal(t, runup.CodeDaemonSkew, refused.Code)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, "run daemon restart", refused.FollowUp.Command)
}

func TestExclusiveStopsTheOtherWorktreesJobsFirst(t *testing.T) {
	w := newWorld(t)
	w.up("api", featB, nil)
	yes := true

	result, _ := w.apply(t, runup.Request{Worktrees: []string{featA}, Exclusive: &yes})

	require.Equal(t, domain.StatusDone, itemOf(t, result.Items, featA).Status)
	assert.Equal(t, []string{w.at(featB)}, w.daemon.stopped)
}

func TestAPortAnotherWorktreeHoldsIsRefusedFollowedByTheExclusiveStart(t *testing.T) {
	w := newWorld(t)
	w.up("api", "main", map[string]int{"PORT": 4000})

	refused := w.refused(t, starting("", verbatim))

	assert.Equal(t, runup.CodePortClash, refused.Code)
	assert.Equal(t, domain.KindRefused, refused.Kind)
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, runup.CodeExclusive, refused.FollowUp.Code)
	assert.Contains(t, refused.FollowUp.Answers, domain.Answered{Path: runup.PathExclusive, Values: []string{"true"}})
}

func TestAJobThatChangesForeignDataIsRefusedUnlessForced(t *testing.T) {
	w := newWorld(t)

	refused := w.refused(t, starting("full", verbatim))
	forced, _ := w.plan(t, runup.Request{Worktrees: []string{verbatim}, Profile: "full", Force: true})

	assert.Equal(t, runup.CodeUnsafe, refused.Code)
	assert.Equal(t, []string{runup.CodeForeignData}, fieldCodes(refused.Fields))
	assert.Equal(t, verbatim, refused.Fields[0].Params["value"], "the problem names the worktree")
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{verbatim}}, {Path: runup.PathProfile, Values: []string{"full"}}, {Path: runup.PathForce, Values: []string{"true"}}}, refused.FollowUp.Answers, "the forced request is the one asked, with nothing resolved added")
	assert.Equal(t, verbatim, forced.Worktrees[0].Worktree)
}

func TestARunnerAndAJobItStartsAreRefusedTogether(t *testing.T) {
	w := newWorld(t)

	refused := w.refused(t, starting("twice", featA))

	assert.Equal(t, []string{runup.CodeRunnerConflict}, fieldCodes(refused.Fields))
}

// A runner up already starts the job the profile would start: the job would run twice.
func TestARunnerAlreadyUpRefusesTheStartOfAJobItRuns(t *testing.T) {
	w := newWorld(t)
	w.up("turbo", featA, nil)

	refused := w.refused(t, starting("", featA))

	assert.Equal(t, []string{runup.CodeRunnerConflict}, fieldCodes(refused.Fields))
	assert.Equal(t, domain.Params{"job": "web", "runner": "turbo"}, refused.Fields[0].Params)
}

func TestAPortNothingAnswersOnIsAWarningOfItsWorktree(t *testing.T) {
	w := newWorld(t)
	w.silent = []int{3010}

	result, _ := w.apply(t, starting("", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusDone, item.Status)
	assert.Equal(t, []string{runup.CodePortSilent}, codes(item.Warnings))
	assert.False(t, *item.Detail.Jobs[1].Ports[0].Listening)
}

func TestACancelLeavesWhatTheDaemonTookAndStartsNothingMore(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(t.Context())
	w.daemon.onStart = func(run.Spec) { cancel() }

	result, _ := w.applyIn(ctx, t, starting("", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusCancelled, item.Status)
	assert.Equal(t, map[string]runup.JobStatus{"api": runup.JobStarted, "web": runup.JobNotStarted}, statusesOf(item))
}

// A start cut short before the daemon took it started nothing: the daemon lists no such job.
func TestAStartCutShortTheDaemonNeverTookIsNotStarted(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(t.Context())
	w.daemon.onStart = func(run.Spec) { cancel() }
	w.daemon.refuse = map[string]error{key(w.at(featA), "api"): context.Canceled}

	result, _ := w.applyIn(ctx, t, starting("", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusCancelled, item.Status)
	assert.Equal(t, map[string]runup.JobStatus{"api": runup.JobNotStarted, "web": runup.JobNotStarted}, statusesOf(item))
}

func TestARepositoryWithNoRunTomlIsRefusedFollowedByRunInit(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, os.Remove(filepath.Join(w.stateDir, "run.toml")))

	refused := w.refused(t, starting("", featA))

	assert.Equal(t, runup.CodeNotInitialized, refused.Code)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	assert.Equal(t, "run init", refused.FollowUp.Command)
}

func TestADaemonThatCannotBeMadeSureOfFailsEveryWorktree(t *testing.T) {
	w := newWorld(t)
	w.daemon.ensureErr = &run.Error{Code: run.CodeLegacy, Message: "the run daemon is the legacy wtm, and it runs 2 job(s)"}

	result, _ := w.apply(t, starting("", featA, featB))

	for _, item := range result.Items {
		assert.Equal(t, domain.StatusFailed, item.Status)
		assert.Equal(t, runup.CodeDaemonSkew, item.Err.Code)
		assert.Equal(t, "run daemon restart", item.Err.FollowUp.Command)
		assert.Equal(t, map[string]runup.JobStatus{"api": runup.JobNotStarted, "web": runup.JobNotStarted}, statusesOf(item), "every job of the plan is listed, none started")
	}
	assert.Empty(t, w.daemon.started)
}

func TestACancelDuringThePortCheckIsInterruptedAndWarnsOfNoPort(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(t.Context())
	w.onListen = cancel

	result, _ := w.applyIn(ctx, t, starting("", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusCancelled, item.Status)
	assert.Equal(t, domain.ReasonInterrupted, item.Reason)
	assert.Empty(t, item.Warnings)
	assert.Nil(t, item.Detail.Jobs[0].Ports[0].Listening, "a check cut short says nothing of a port")
}

func TestAJobRunTomlKeepsOutOfTheCheckIsNeitherCheckedNorWarnedOf(t *testing.T) {
	w := newWorld(t)
	w.silent = []int{4010, 7010}

	result, _ := w.apply(t, starting("probed", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, []string{runup.CodePortSilent}, codes(item.Warnings))
	assert.Equal(t, "api", item.Warnings[0].Params["job"])
	assert.Nil(t, item.Detail.Jobs[1].Ports[0].Listening, "probe = false")
}

// A runner up in one worktree and its child up in another: no worktree runs a job twice.
func TestARunnerAndItsChildUpInTwoWorktreesAreNoConflict(t *testing.T) {
	w := newWorld(t)
	w.up("turbo", featA, nil)
	w.up("web", featB, nil)

	_, _, err := w.usecase.Plan(t.Context(), starting("alone", featA, featB))

	assert.NoError(t, err)
}

func TestAPortAnotherRepositoryHoldsIsRefusedWithNoExclusiveStart(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: "api", WorkDir: "/elsewhere/repo", Up: true, State: run.Running, Hold: run.HoldOwn, Ports: map[string]int{"PORT": 4010}})
	yes := true

	refused := w.refused(t, starting("", featA))
	exclusive := w.refused(t, runup.Request{Worktrees: []string{featA}, Exclusive: &yes})

	assert.Equal(t, runup.CodePortClash, refused.Code)
	assert.Nil(t, refused.FollowUp, "stopping this repository's worktrees frees no port of another")
	assert.Contains(t, refused.Message, "/elsewhere/repo")
	assert.Equal(t, "{worktree}'s {job} would bind {port}, which {held_job}, a job outside this repository's worktrees, holds in {held_by}", refused.Fields[0].Template, "the holder travels in the params")
	assert.Equal(t, "/elsewhere/repo", refused.Fields[0].Params["held_by"])
	assert.Equal(t, runup.CodePortClash, exclusive.Code)
}

func TestExclusiveLiftsAClashWithAnotherWorktree(t *testing.T) {
	w := newWorld(t)
	w.up("api", "main", map[string]int{"PORT": 4000})
	yes := true

	planned, _ := w.plan(t, runup.Request{Worktrees: []string{verbatim}, Exclusive: &yes})

	assert.Equal(t, []string{"main"}, []string{planned.Others[0].Worktree})
}

func TestADaemonOfAnotherBuildHoldingNoJobIsNoRefusal(t *testing.T) {
	w := newWorld(t)
	w.daemon.skewed = &run.Skewed{Skew: run.SkewLegacy}

	_, _, err := w.usecase.Plan(t.Context(), starting("", featA))

	assert.NoError(t, err, "Ensure replaces it")
}

func TestTheRecapListsTheJobsOfEachWorktree(t *testing.T) {
	w := newWorld(t)
	w.up("api", featB, nil)

	planned, _ := w.plan(t, starting("", featA, featB))

	var labels []string
	for _, row := range planned.Recap {
		labels = append(labels, row.Label)
	}
	assert.Equal(t, []string{"Worktrees", "Profile", "Jobs in " + featA, "Jobs in " + featB}, labels)
	assert.Equal(t, "api (up)", planned.Recap[3].Values[0].Value)
}

func TestAWorktreeWhoseNumberingFailsFailsAlone(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.stateDir, "worktrees", "feat%2Fb", "meta.json"), "not json")

	result, _ := w.apply(t, starting("", fresh))

	item := itemOf(t, result.Items, fresh)
	assert.Equal(t, domain.StatusFailed, item.Status)
	assert.Equal(t, runup.CodeUnnumbered, item.Err.Code)
	assert.Equal(t, runup.JobNotStarted, item.Detail.Jobs[0].Status)
}

func TestAWorktreeWhoseJobsCouldNotStopIsAWarning(t *testing.T) {
	w := newWorld(t)
	w.up("api", featB, nil)
	w.daemon.stopErr = errors.New("connection reset")
	yes := true

	result, _ := w.apply(t, runup.Request{Worktrees: []string{featA}, Exclusive: &yes})

	assert.Equal(t, []string{runup.CodeStopFailed}, codes(result.Warnings))
	assert.Equal(t, domain.StatusDone, itemOf(t, result.Items, featA).Status)
}

func TestANamespaceTheRecordCannotKeepIsAWarning(t *testing.T) {
	w := newWorld(t)
	w.usecase.Meta = unwritable{w.meta}

	result, _ := w.apply(t, starting("full", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusDone, item.Status)
	assert.Equal(t, []string{runup.CodeNamespaceUnrecorded}, codes(item.Warnings))
}

func TestTheSettleCatchesACrashWithinItAndLeavesOneAfterIt(t *testing.T) {
	for name, c := range map[string]struct {
		settle, crash time.Duration
		want          runup.JobStatus
	}{
		"within": {settle: 300 * time.Millisecond, crash: 50 * time.Millisecond, want: runup.JobCrashed},
		"after":  {settle: 50 * time.Millisecond, crash: 2 * time.Second, want: runup.JobStarted},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.usecase.Settle = c.settle
			w.daemon.late = map[string]time.Duration{key(w.at(featA), "api"): c.crash}

			result, _ := w.apply(t, runup.Request{Worktrees: []string{featA}, Profile: "alone", NoProbe: true})

			assert.Equal(t, c.want, itemOf(t, result.Items, featA).Detail.Jobs[0].Status)
		})
	}
}

// The daemon took the join before the cancel: its namespace exists, and clean must find it in the record.
func TestAJoinCutShortTheDaemonTookRecordsItsNamespace(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(t.Context())
	w.daemon.onStart = func(spec run.Spec) {
		if spec.Job.Name == "db" {
			cancel()
		}
	}
	w.daemon.cut = map[string]bool{key(w.at(featA), "db"): true}

	result, _ := w.applyIn(ctx, t, starting("full", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusCancelled, item.Status)
	assert.Equal(t, runup.JobJoined, statusesOf(item)["db"])
	assert.Equal(t, []string{"db"}, w.stored(t, featA).Namespaces)
}

// A task cut short that ran to its end before the daemon answered is listed ended, no longer up: it ran, and its exit code says how.
func TestATaskCutShortThatRanToItsEndIsDone(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(t.Context())
	w.daemon.onStart = func(spec run.Spec) {
		if spec.Job.Name == "migrate" {
			cancel()
		}
	}
	w.daemon.cut = map[string]bool{key(w.at(featA), "migrate"): true}

	result, _ := w.applyIn(ctx, t, starting("full", featA))

	item := itemOf(t, result.Items, featA)
	assert.Equal(t, domain.StatusCancelled, item.Status)
	assert.Equal(t, map[string]runup.JobStatus{"migrate": runup.JobDone, "db": runup.JobNotStarted, "api": runup.JobNotStarted, "web": runup.JobNotStarted}, statusesOf(item))
}

func TestTheOutcomesPlanOfAWorktreeTheStartNumberedNamesItsPorts(t *testing.T) {
	w := newWorld(t)

	outcome, err := w.usecase.Run(t.Context(), starting("full", fresh), domain.RunOptions{})

	require.NoError(t, err)
	planned := outcome.Plan.Worktrees[0]
	assert.True(t, planned.Numbered)
	assert.Equal(t, map[string]int{"PORT": 4030}, planned.Jobs[2].Ports)
	assert.Equal(t, "app_feat-fresh", planned.Jobs[1].Namespace)
}
