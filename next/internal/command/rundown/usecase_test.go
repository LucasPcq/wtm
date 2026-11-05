package rundown_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStopPlansEachJobsFateAndTheSharedServicesKeptForOthers(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "full")
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: "cache", WorkDir: w.at(featA), Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldDetached, Profile: "full", StopCommand: "docker compose down --volumes"})
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: "web", WorkDir: w.at(featA), Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn})
	w.shared(false, featA, featB)

	planned, _ := w.plan(t, w.stopping(featA))

	require.Len(t, planned.Worktrees, 1)
	assert.Equal(t, rundown.Target{Profile: "full", StopTarget: runjobs.StopTarget{Worktree: featA, Branch: featA, Path: w.at(featA), Jobs: []runjobs.StopJob{
		{Name: "api", Kind: project.Service, Hold: run.HoldOwn, Fate: run.FateStop},
		{Name: "cache", Kind: project.Service, Hold: run.HoldDetached, Fate: run.FateStop, StopCommand: "docker compose down --volumes"},
		{Name: "web", Kind: project.Service, Hold: run.HoldOwn, Fate: run.FateClear},
		{Name: "db", Kind: project.Service, Hold: run.HoldClaim, Fate: run.FateRelease, KeptBy: []string{featB}, SharedIn: "main"},
	}}}, planned.Worktrees[0])
	assert.False(t, planned.Empty)
}

func TestTheLastHoldsOnASharedServiceStopIt(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA, featB)

	planned, _ := w.plan(t, w.stopping(featA, featB))

	for _, target := range planned.Worktrees {
		assert.Equal(t, []runjobs.StopJob{{Name: "db", Kind: project.Service, Hold: run.HoldClaim, Fate: run.FateStop, SharedIn: "main"}}, target.Jobs, target.Worktree)
	}
}

func TestAProfileNarrowsTheStopAndNamesItsJobsNotRunning(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	w.up("cache", featA, "")

	planned, _ := w.plan(t, rundown.Request{Worktrees: []string{featA}, Profile: "dev"})

	assert.Equal(t, []runjobs.StopJob{{Name: "api", Kind: project.Service, Hold: run.HoldOwn, Fate: run.FateStop}}, planned.Worktrees[0].Jobs)
	assert.Equal(t, []string{"web"}, planned.Worktrees[0].NotRunning)
}

func TestNothingRunningIsAnEmptyPlan(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, w.stopping(featA))

	assert.True(t, planned.Empty)
	assert.Equal(t, []runjobs.StopJob{}, planned.Worktrees[0].Jobs)
}

func TestAllStopsEveryWorktreeOfTheRepositoryRunningSomething(t *testing.T) {
	w := newWorld(t)
	w.up("api", featB, "dev")
	w.up("web", "main", "dev")
	w.daemon.listed = append(w.daemon.listed, service("api", "/elsewhere/repo", "dev"))

	planned, _ := w.plan(t, rundown.Request{All: true})

	var names []string
	for _, target := range planned.Worktrees {
		names = append(names, target.Worktree)
	}
	assert.Equal(t, []string{"main", featB}, names)
}

func TestARequestsProblemsAreRefusedAsFieldErrors(t *testing.T) {
	cases := map[string]struct {
		req  rundown.Request
		code string
		path string
	}{
		"not_found":    {req: rundown.Request{Worktrees: []string{"nope"}}, code: domain.CodeNotFound, path: "worktree[0]"},
		"all_narrowed": {req: rundown.Request{Worktrees: []string{featA}, All: true}, code: rundown.CodeAllNarrowed, path: rundown.PathAll},
		"profile":      {req: rundown.Request{Worktrees: []string{featA}, Profile: "ghost"}, code: domain.CodeNotFound, path: rundown.PathProfile},
		"no_worktree":  {req: rundown.Request{}, code: domain.CodeRequired, path: domain.PathWorktree},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)

			refused := w.refused(t, c.req)

			require.NotEmpty(t, refused.Fields)
			assert.Equal(t, c.code, refused.Fields[0].Code)
			assert.Equal(t, c.path, refused.Fields[0].Path)
		})
	}
}

func TestTheCallersWorktreeIsStoppedWhenNoneIsNamed(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")

	planned, _ := w.plan(t, rundown.Request{}.From(domain.Caller{Cwd: w.at(featA)}))

	assert.Equal(t, featA, planned.Worktrees[0].Worktree)
}

func TestARemovedWorktreesPathIsStoppedWhileItsJobsRun(t *testing.T) {
	w := newWorld(t)
	removed := w.root + "/wt-removed"
	left := service("api", removed, "dev")
	left.Repo = w.main
	w.daemon.listed = append(w.daemon.listed, left)

	result, _ := w.apply(t, w.stopping(removed))

	require.Len(t, result.Items, 1)
	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Equal(t, []string{key(removed, "api")}, w.daemon.stopped)
	assert.Empty(t, result.FollowUps)
}

func TestAStopThatFailsIsThatJobsAndTheOthersAreReported(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	w.up("cache", featA, "dev")
	w.up("web", featB, "dev")
	w.daemon.refuse = map[string]error{key(w.at(featA), "cache"): errStack}

	result, _ := w.apply(t, w.stopping(featA, featB))

	require.Len(t, result.Items, 2)
	failed := result.Items[0]
	assert.Equal(t, domain.StatusFailed, failed.Status)
	assert.Equal(t, rundown.CodeStopFailed, failed.Err.Code)
	assert.Equal(t, "cache", failed.Err.Params["jobs"])
	assert.Equal(t, map[string]runjobs.StopStatus{"api": runjobs.StopStopped, "cache": runjobs.StopFailure}, statuses(failed))
	assert.Equal(t, domain.StatusDone, result.Items[1].Status)
}

func TestEachStopIsAPhaseOfItsWorktreesUnit(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")

	_, report := w.apply(t, w.stopping(featA))

	assert.Equal(t, []domain.Event{
		domain.UnitStarted{Subject: featA},
		domain.PhaseStarted{Subject: featA, Phase: domain.PhaseJob, Job: "api"},
		domain.PhaseFinished{Subject: featA, Phase: domain.PhaseJob, Job: "api"},
		domain.UnitFinished{Subject: featA, Status: domain.StatusDone},
	}, report.All())
}

func TestAReleaseACrashClearedAndAWorktreeRunningNothingAreReported(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA, featB)
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: "web", WorkDir: w.at(featA), Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn})
	w.daemon.outcomes = map[string]run.Outcome{key(w.at(featA), "db"): run.OutcomeReleased, key(w.at(featA), "web"): run.OutcomeNotRunning}

	result, report := w.apply(t, w.stopping(featA, "main"))

	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopReleased, "web": runjobs.StopCleared}, statuses(result.Items[0]))
	unchanged := result.Items[1]
	assert.Equal(t, domain.StatusDone, unchanged.Status)
	assert.Equal(t, domain.ReasonUnchanged, unchanged.Reason)
	assert.NotContains(t, report.All(), domain.UnitStarted{Subject: "main"})
}

func TestTheSharedHoldsAreLetGoOfAfterTheWorktreesOwnJobs(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA)
	w.up("api", featA, "full")
	w.up("web", featA, "full")

	w.apply(t, w.stopping(featA))

	assert.Equal(t, key(w.at(featA), "db"), w.daemon.stopped[len(w.daemon.stopped)-1])
}

func TestAStopCancelledLeavesTheStopsNotSentNotReached(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA)
	w.up("api", featA, "full")
	ctx, cancel := context.WithCancel(t.Context())
	w.daemon.onStop = cancel

	result, _ := w.applyIn(ctx, t, w.stopping(featA))

	item := result.Items[0]
	assert.Equal(t, domain.StatusCancelled, item.Status)
	assert.Equal(t, map[string]runjobs.StopStatus{"api": runjobs.StopStopped, "db": runjobs.StopNotReached}, statuses(item))
}

func TestTheFollowUpStartsAgainWhatStoppedWithTheProfileItRan(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	w.up("api", featB, "dev")

	result, _ := w.apply(t, w.stopping(featA, featB, "main"))

	assert.Equal(t, []domain.FollowUp{{Command: domain.CommandRunUp, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA, featB}}, {Path: "profile", Values: []string{"dev"}}}, Code: rundown.CodeUp, Message: "start them again"}}, result.FollowUps)
}

func TestADaemonThatDiedLeavingProcessesIsStartedToTakeThemOver(t *testing.T) {
	w := newWorld(t)
	w.daemon.unknown = true
	w.up("api", featA, "dev")

	planned, _ := w.plan(t, w.stopping(featA))

	assert.True(t, w.daemon.ensured)
	assert.Len(t, planned.Worktrees[0].Jobs, 1)
}

func TestAnInvalidRunTomlStopsWhatRunsAllTheSame(t *testing.T) {
	w := newWorld(t)
	writeFile(t, w.stateDir+"/run.toml", "[[job]\n")
	w.up("api", featA, "dev")

	planned, warnings := w.plan(t, w.stopping(featA))

	assert.Len(t, planned.Worktrees[0].Jobs, 1)
	require.Len(t, warnings, 1)
	assert.Equal(t, rundown.CodeRunInvalid, warnings[0].Code)
}

func TestADaemonOfAnotherBuildIsWarnedOfAndAskedAllTheSame(t *testing.T) {
	w := newWorld(t)
	w.daemon.skewed = &run.Skewed{Skew: run.SkewBuild, Daemon: "0.30.0"}
	w.up("api", featA, "dev")

	planned, warnings := w.plan(t, w.stopping(featA))

	assert.Len(t, planned.Worktrees[0].Jobs, 1)
	require.Len(t, warnings, 1)
	assert.Equal(t, rundown.CodeDaemonSkew, warnings[0].Code)
	require.NotNil(t, warnings[0].FollowUp)
	assert.Equal(t, "run daemon restart", warnings[0].FollowUp.Command)
}

func TestTheFollowUpOfAStopNarrowedToAProfileStartsThatProfile(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "full")

	result, _ := w.apply(t, rundown.Request{Worktrees: []string{featA}, Profile: "dev"})

	require.Len(t, result.FollowUps, 1)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA}}, {Path: "profile", Values: []string{"dev"}}}, result.FollowUps[0].Answers)
}

// Review: a listed worktree named by its path is that worktree, its lock and its name; another repository's directory is none of this one's.
func TestAPathNamesItsWorktreeOrARemovedOneOfThisRepositoryAlone(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	foreign := service("api", "/other/repo/wt-x", "dev")
	foreign.Repo = "/other/repo"
	w.daemon.listed = append(w.daemon.listed, foreign)

	planned, _ := w.plan(t, w.stopping(w.at(featA)))
	refused := w.refused(t, w.stopping("/other/repo/wt-x"))

	assert.Equal(t, featA, planned.Worktrees[0].Worktree)
	assert.False(t, planned.Worktrees[0].Removed)
	assert.Equal(t, domain.WorktreeLocks([]string{featA}), planned.Locks())
	assert.Equal(t, domain.CodeNotFound, refused.Fields[0].Code)
}

func TestTheLocksAreTheWorktreesPlannedARemovedPathAndAllIncluded(t *testing.T) {
	w := newWorld(t)
	removed := w.root + "/wt-removed"
	left := service("api", removed, "dev")
	left.Repo = w.main
	w.daemon.listed = append(w.daemon.listed, left)
	w.up("api", featB, "dev")

	byPath, _ := w.plan(t, w.stopping(removed))
	all, _ := w.plan(t, rundown.Request{All: true})

	assert.Equal(t, domain.WorktreeLocks([]string{removed}), byPath.Locks())
	assert.Equal(t, domain.WorktreeLocks([]string{featB, removed}), all.Locks())
}

// Review: the form is a read, which never starts a daemon; the plan starts one to reach the processes its index holds.
func TestTheQuestionsLeaveTheDaemonAloneThePlanStartsOneToReachItsIndex(t *testing.T) {
	w := newWorld(t)
	w.daemon.unknown = true
	w.up("api", featA, "dev")

	_, err := w.usecase.Questions(t.Context(), w.stopping(featA))

	require.NoError(t, err)
	assert.False(t, w.daemon.ensured)
	w.plan(t, w.stopping(featA))
	assert.True(t, w.daemon.ensured)
}

// Review: a job of the profile up for the others in its worktree, a shared service main runs for feat/b's hold, is not "not running".
func TestAProfilesJobUpForOthersIsNotSaidNotRunning(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featB)
	w.up("api", "main", "full")

	planned, _ := w.plan(t, rundown.Request{Worktrees: []string{"main"}, Profile: "full"})

	assert.Equal(t, []string{"cache", "web"}, planned.Worktrees[0].NotRunning)
}

// Review: two holds let go of together stop the service whichever release lands first; the planned fate is reported.
func TestHoldsLetGoOfTogetherReportThePlannedStop(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA, featB)
	w.daemon.outcomes = map[string]run.Outcome{key(w.at(featA), "db"): run.OutcomeReleased}

	result, _ := w.apply(t, w.stopping(featA, featB))

	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopStopped}, statuses(result.Items[0]))
	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopStopped}, statuses(result.Items[1]))
}

func TestMainLetsGoOfASharedServiceItHoldsForTheOthersToKeep(t *testing.T) {
	w := newWorld(t)
	w.shared(true, featA)
	w.daemon.outcomes = map[string]run.Outcome{key(w.main, "db"): run.OutcomeReleased}

	planned, _ := w.plan(t, w.stopping("main"))
	result, _ := w.apply(t, w.stopping("main"))

	assert.Equal(t, []runjobs.StopJob{{Name: "db", Kind: project.Service, Hold: run.HoldOwn, Fate: run.FateRelease, KeptBy: []string{featA}, SharedIn: "main"}}, planned.Worktrees[0].Jobs)
	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopReleased}, statuses(result.Items[0]))
}

// Review: a daemon of another build may not say whether main holds a shared service: the fates are warned approximate when a worktree stopped holds one the index has no record of, one the plan leaves out included; its stop's outcome is taken as it answered.
func TestASkewedDaemonsSharedFatesAreWarnedApproximateAndItsOutcomesRead(t *testing.T) {
	w := newWorld(t)
	w.daemon.skewed = &run.Skewed{Skew: run.SkewLegacy}
	w.shared(false, featA, featB)
	w.daemon.listed[0].MainHoldsUnknown = true
	w.daemon.outcomes = map[string]run.Outcome{key(w.at(featA), "db"): run.OutcomeReleased}

	_, warnings := w.plan(t, w.stopping(featA))
	_, leftOut := w.plan(t, w.stopping("main"))
	result, _ := w.apply(t, w.stopping(featA))

	assert.Equal(t, []string{rundown.CodeDaemonSkew, rundown.CodeFatesApproximate}, []string{warnings[0].Code, warnings[1].Code})
	assert.Equal(t, []string{rundown.CodeDaemonSkew, rundown.CodeFatesApproximate}, []string{leftOut[0].Code, leftOut[1].Code})
	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopReleased}, statuses(result.Items[0]))
}

// Review: holds let go of together whose service another worktree came to hold meanwhile were each released.
func TestHoldsLetGoOfTogetherWhileAnotherCameToHoldItReportReleased(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA, featB)
	w.daemon.outcomes = map[string]run.Outcome{key(w.at(featA), "db"): run.OutcomeReleased, key(w.at(featB), "db"): run.OutcomeReleased}

	result, _ := w.apply(t, w.stopping(featA, featB))

	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopReleased}, statuses(result.Items[0]))
	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopReleased}, statuses(result.Items[1]))
}

// Review: a release planned whose others let go meanwhile stopped the service: it says so.
func TestAReleaseThatStoppedTheServiceSaysStopped(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA, featB)

	result, _ := w.apply(t, w.stopping(featA))

	assert.Equal(t, map[string]runjobs.StopStatus{"db": runjobs.StopStopped}, statuses(result.Items[0]))
}

// Review: a daemon started that still cannot tell which jobs of its index are up is refused, not planned as though none were.
func TestADaemonStartedThatStillCannotTellIsRefused(t *testing.T) {
	w := newWorld(t)
	w.daemon.unknown, w.daemon.staysUnknown = true, true
	w.up("api", featA, "dev")

	refused := w.refused(t, w.stopping(featA))

	assert.Equal(t, rundown.CodeDaemonUnavailable, refused.Code)
}

// Review: a path is read from where the caller stands and through its symbolic links, spelled as git spells the worktree's root.
func TestAPathIsResolvedFromTheCallerToItsWorktreesRoot(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	alias := filepath.Join(gittest.TempDir(t), "alias")
	require.NoError(t, os.Symlink(w.root, alias))
	cases := map[string]struct {
		cwd  string
		name string
	}{
		"dot":            {cwd: w.at(featA), name: "."},
		"relative":       {cwd: w.main, name: "../wt-feat-a"},
		"symlink":        {cwd: w.main, name: filepath.Join(alias, "wt-feat-a")},
		"trailing_slash": {cwd: w.main, name: w.at(featA) + "/"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			planned, _ := w.plan(t, rundown.Request{Worktrees: []string{c.name}}.From(domain.Caller{Cwd: c.cwd}))

			require.Len(t, planned.Worktrees, 1)
			assert.Equal(t, featA, planned.Worktrees[0].Worktree)
			assert.Equal(t, domain.WorktreeLocks([]string{featA}), planned.Locks())
		})
	}

	twice := w.refused(t, rundown.Request{Worktrees: []string{featA, "."}}.From(domain.Caller{Cwd: w.at(featA)}))
	assert.Equal(t, domain.CodeDistinct, twice.Fields[0].Code)
}

// Review: a removed worktree's path, named through a symbolic link, is the path its jobs run in, and its lock.
func TestARemovedWorktreesPathIsResolvedThroughItsLinks(t *testing.T) {
	w := newWorld(t)
	removed := w.root + "/wt-removed"
	left := service("api", removed, "dev")
	left.Repo = w.main
	w.daemon.listed = append(w.daemon.listed, left)
	alias := filepath.Join(gittest.TempDir(t), "alias")
	require.NoError(t, os.Symlink(w.root, alias))

	planned, _ := w.plan(t, w.stopping(filepath.Join(alias, "wt-removed")))

	require.Len(t, planned.Worktrees, 1)
	assert.Equal(t, rundown.Target{StopTarget: runjobs.StopTarget{Worktree: removed, Path: removed, Removed: true, Jobs: []runjobs.StopJob{{Name: "api", Kind: project.Service, Hold: run.HoldOwn, Fate: run.FateStop}}}, Profile: "dev"}, planned.Worktrees[0])
	assert.Equal(t, domain.WorktreeLocks([]string{removed}), planned.Locks())
}

// Review: a daemon of another build whose shared services the index records says who holds them: no fate is approximate.
func TestASkewedDaemonsRecordedSharedServicesAreNoApproximation(t *testing.T) {
	w := newWorld(t)
	w.daemon.skewed = &run.Skewed{Skew: run.SkewLegacy}
	w.shared(true, featA)

	planned, warnings := w.plan(t, w.stopping(featA))

	require.Len(t, warnings, 1)
	assert.Equal(t, rundown.CodeDaemonSkew, warnings[0].Code)
	assert.Equal(t, []string{"main"}, planned.Worktrees[0].Jobs[0].KeptBy)
}

// Review: a daemon of another protocol, or the legacy one, that the client cannot go on with refuses the stop as a precondition followed by the restart, whether reading the jobs or making sure of a daemon.
func TestADaemonTheClientCannotGoOnWithIsRefusedFollowedByItsRestart(t *testing.T) {
	skew := &run.Error{Code: run.CodeSkew, Message: "the run daemon speaks protocol 1, this wtm 2"}
	cases := map[string]func(d *daemon){
		"jobs":   func(d *daemon) { d.jobsErr = skew },
		"ensure": func(d *daemon) { d.unknown, d.ensureErr = true, skew },
	}
	for name, setUp := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			setUp(w.daemon)

			refused := w.refused(t, w.stopping(featA))

			assert.Equal(t, domain.KindPrecondition, refused.Kind)
			assert.Equal(t, rundown.CodeDaemonSkew, refused.Code)
			require.NotNil(t, refused.FollowUp)
			assert.Equal(t, "run daemon restart", refused.FollowUp.Command)
		})
	}
}

// Review: --all reaches the worktrees removed whose jobs still run for this repository, by path, after the listed ones.
func TestAllStopsTheRemovedWorktreesJobsToo(t *testing.T) {
	w := newWorld(t)
	w.up("api", featA, "dev")
	removed := w.root + "/wt-removed"
	w.removedWith("api", removed)
	foreign := service("api", "/other/repo/wt-x", "dev")
	foreign.Repo = "/other/repo"
	w.daemon.listed = append(w.daemon.listed, foreign)

	planned, _ := w.plan(t, rundown.Request{All: true})

	require.Len(t, planned.Worktrees, 2)
	assert.Equal(t, featA, planned.Worktrees[0].Worktree)
	assert.Equal(t, rundown.Target{StopTarget: runjobs.StopTarget{Worktree: removed, Path: removed, Removed: true, Jobs: []runjobs.StopJob{{Name: "api", Kind: project.Service, Hold: run.HoldOwn, Fate: run.FateStop}}}, Profile: "dev"}, planned.Worktrees[1])
}

// Review: a crashed job that started again before the stop was stopped, not cleared.
func TestACrashThatStartedAgainIsReportedStopped(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: "web", WorkDir: w.at(featA), Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn})

	result, _ := w.apply(t, w.stopping(featA))

	assert.Equal(t, map[string]runjobs.StopStatus{"web": runjobs.StopStopped}, statuses(result.Items[0]))
}

// Review: a worktree whose only jobs stopped are holds on shared services is started again under their profile.
func TestTheFollowUpOfHoldsAloneStartsTheirProfile(t *testing.T) {
	w := newWorld(t)
	w.shared(true, featA)

	result, _ := w.apply(t, w.stopping(featA))

	require.Len(t, result.FollowUps, 1)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA}}, {Path: "profile", Values: []string{"full"}}}, result.FollowUps[0].Answers)
}

// Review: a profile asked while run.toml does not load is said so, not said missing from it.
func TestAProfileAskedWhileRunTomlDoesNotLoadSaysSo(t *testing.T) {
	w := newWorld(t)
	writeFile(t, w.stateDir+"/run.toml", "[[job]\n")

	refused := w.refused(t, rundown.Request{Worktrees: []string{featA}, Profile: "dev"})

	assert.Contains(t, refused.Fields[0].Template, "run.toml does not load")
}

// Review: a path is locked once resolved, never under the spelling asked.
func TestTheRequestLocksOnlyTheBranchesItNames(t *testing.T) {
	req := rundown.Request{Worktrees: []string{featA, ".", "../wt-feat-b", "/abs/wt-x", "trailing/"}}

	assert.Equal(t, domain.WorktreeLocks([]string{featA}), req.Locks())
}

// Review: a path the run daemon runs jobs at without saying which repository they were started for is refused saying so.
func TestAPathOfJobsOfNoKnownRepositoryIsRefusedSayingSo(t *testing.T) {
	w := newWorld(t)
	removed := w.root + "/wt-removed"
	w.daemon.listed = append(w.daemon.listed, service("api", removed, "dev"))

	refused := w.refused(t, w.stopping(removed))

	assert.Equal(t, domain.CodeNotFound, refused.Fields[0].Code)
	assert.Contains(t, refused.Fields[0].Template, "without saying which repository")
}

// Review: a shared service main runs for the others alone gives main no profile to start again.
func TestASharedServiceRunForTheOthersGivesItsCheckoutNoProfile(t *testing.T) {
	w := newWorld(t)
	w.shared(false, featA)
	w.daemon.listed[0].Profile = "full"

	planned, _ := w.plan(t, rundown.Request{All: true})

	require.Len(t, planned.Worktrees, 2)
	assert.Equal(t, "main", planned.Worktrees[0].Worktree)
	assert.Empty(t, planned.Worktrees[0].Profile)
	assert.Equal(t, "full", planned.Worktrees[1].Profile)
}
