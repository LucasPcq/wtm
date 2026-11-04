package runlogs_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachJobThatRanIsReadWithItsLastLinesInRunTomlsOrder(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "migrate", "migrating", "migrated")
	w.logged(t, featA, "api", "api ready", "GET /health 200")
	w.logged(t, featA, "crashy", "starting", "boom")
	crashed := ran("crashy", w.at(featA), run.Crashed)
	crashed.ExitCode = new(1)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running), crashed}

	read := w.read(t, runlogs.Request{Worktrees: []string{featA}})

	require.Len(t, read.Items, 1)
	item := read.Items[0]
	assert.Equal(t, domain.StatusDone, item.Status)
	assert.Equal(t, []string{"api", "migrate", "crashy"}, namesOf(item), "a job that never ran is left out")
	api := jobOf(t, item, "api")
	assert.Equal(t, runlogs.JobRunning, api.State)
	assert.Equal(t, run.HoldOwn, api.Hold)
	assert.Equal(t, "http://api.localhost", api.URL)
	assert.Equal(t, []string{"api ready", "GET /health 200"}, textsOf(api), "a line from the log is its text alone, its stamp in at")
	assert.Equal(t, at, api.Lines[0].At)
	assert.Equal(t, w.stateDir+"/logs/feat%2Fa/api.log", api.Log)
	assert.Equal(t, runlogs.JobStopped, jobOf(t, item, "migrate").State, "a job only its log knows has stopped")
	assert.Equal(t, runlogs.JobCrashed, jobOf(t, item, "crashy").State)
	assert.Equal(t, new(1), jobOf(t, item, "crashy").ExitCode)
}

func TestAWorktreeWhereNothingRanHasNoJob(t *testing.T) {
	w := newWorld(t)

	read := w.read(t, runlogs.Request{Worktrees: []string{featB}})

	require.Len(t, read.Items, 1)
	assert.Equal(t, domain.StatusDone, read.Items[0].Status)
	assert.Empty(t, read.Items[0].Detail.Jobs)
}

func TestAJobNamedThatNeverRanIsListedNotStarted(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "api", "api ready")

	read := w.read(t, runlogs.Request{Worktrees: []string{featA}, Job: "never"})

	assert.Equal(t, []string{"never"}, namesOf(read.Items[0]), "the job named alone")
	assert.Equal(t, runlogs.JobNotStarted, jobOf(t, read.Items[0], "never").State)
	assert.Empty(t, jobOf(t, read.Items[0], "never").Lines)
}

func TestARequestRunTomlCannotAnswerIsRefusedByItsField(t *testing.T) {
	for name, c := range map[string]struct {
		req  runlogs.Request
		code string
	}{
		"job_undeclared":   {req: runlogs.Request{Worktrees: []string{featA}, Job: "ghost"}, code: runlogs.CodeJobUnknown},
		"worktree_unknown": {req: runlogs.Request{Worktrees: []string{"feat"}}, code: domain.CodeNotFound},
		"worktree_gone":    {req: runlogs.Request{Worktrees: []string{gone}}, code: runlogs.CodeWorktreeGone},
		"nowhere":          {req: runlogs.Request{}, code: domain.CodeRequired},
	} {
		t.Run(name, func(t *testing.T) {
			failure := newWorld(t).refused(t, c.req)
			assert.Equal(t, domain.KindInvalid, failure.Kind)
			assert.Equal(t, []string{c.code}, fieldCodes(failure.Fields))
		})
	}
}

func TestNoWorktreeNamedReadsTheOneTheCallerStandsIn(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "api", "api ready")

	read := w.read(t, runlogs.Request{Caller: domain.Caller{Cwd: w.at(featA)}})

	require.Len(t, read.Items, 1)
	assert.Equal(t, featA, read.Items[0].Subject)
}

func TestARepositoryWithNoRunTomlIsRefused(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, removeFile(w.stateDir+"/run.toml"))

	failure := w.refused(t, runlogs.Request{Worktrees: []string{featA}})

	assert.Equal(t, domain.KindPrecondition, failure.Kind)
	assert.Equal(t, runlogs.CodeNotInitialized, failure.Code)
}

func TestWithNoDaemonTheLinesComeFromTheLogs(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "api", "api ready")
	w.daemon.jobsErr = run.ErrUnknown

	read := w.read(t, runlogs.Request{Worktrees: []string{featA}})

	assert.Equal(t, []string{"api ready"}, textsOf(jobOf(t, read.Items[0], "api")))
	assert.Equal(t, []string{runlogs.Name + domain.JobsUnknown}, codes(read.Warnings))
	assert.Empty(t, w.daemon.asked, "the logs are read, the daemon is never asked for lines")
}

func TestADaemonOfAnotherBuildIsWarnedWithItsRestart(t *testing.T) {
	w := newWorld(t)
	w.daemon.skewed = &run.Skewed{Skew: run.SkewLegacy, Daemon: "0.30.1"}

	read := w.read(t, runlogs.Request{Worktrees: []string{featA}})

	require.Equal(t, []string{runlogs.Name + domain.DaemonSkew}, codes(read.Warnings))
	assert.NotNil(t, read.Warnings[0].FollowUp)
}

func TestAWorktreeHoldingASharedServiceReadsItsLinesInTheMainCheckout(t *testing.T) {
	w := newWorld(t)
	w.logged(t, "main", "db", "db listening")
	db := ran("db", w.main, run.Running)
	db.SharedDir = w.main
	claim := run.Job{Name: "db", WorkDir: w.at(featA), Up: true, Hold: run.HoldClaim, SharedDir: w.main}
	w.daemon.listed = []run.Job{db, claim}

	read := w.read(t, runlogs.Request{Worktrees: []string{featA, featB, "main"}})

	held := jobOf(t, read.Items[0], "db")
	assert.Equal(t, run.HoldClaim, held.Hold)
	assert.Equal(t, "main", held.SharedIn)
	assert.Equal(t, runlogs.JobRunning, held.State, "the state of the service it holds")
	assert.Equal(t, []string{"db listening"}, textsOf(held))
	assert.Empty(t, read.Items[1].Detail.Jobs, "a worktree holding no claim does not show the shared service")
	assert.Equal(t, []string{"db listening"}, textsOf(jobOf(t, read.Items[2], "db")))
}

func TestWatchWithoutFollowStreamsEachJobsLastLinesThenEnds(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "api", "api ready")
	w.logged(t, featA, "migrate", "migrated")
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}

	got, _, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/a api api ready", "feat/a migrate migrated"}, got)
	assert.Empty(t, w.daemon.asked, "without follow the daemon is not asked")
}

func TestWatchFollowsARunningJobUntilItEnds(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "migrate", "migrated")
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: lines("api ready", "GET / 200")}}

	got, _, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}, Follow: true})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"feat/a api api ready", "feat/a api GET / 200", "feat/a migrate migrated"}, got)
	require.Len(t, w.daemon.asked, 1)
	assert.Equal(t, run.Tail{Name: "api", WorkDir: w.at(featA), Lines: run.MaxTail, Follow: true}, w.daemon.asked[0])
}

func TestAFollowerThatFellBehindFollowsAgainWithoutRepeatingALine(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	all := lines("one", "two", "three", "four", "five")
	behind := &run.Error{Code: run.CodeFollowerBehind}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: all[:3], err: behind}, {lines: all[1:]}}

	got, warned, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}, Job: "api", Follow: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/a api one", "feat/a api two", "feat/a api three", "feat/a api four", "feat/a api five"}, got)
	assert.Empty(t, warned)
}

func TestAFollowerThatLostLinesSaysSo(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	all := lines("one", "two", "three", "four")
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: all[:1], err: &run.Error{Code: run.CodeFollowerBehind}}, {lines: all[2:]}}
	// "one" rotated out of what the daemon keeps: "two" was never sent

	got, warned, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}, Job: "api", Follow: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/a api one", "feat/a api three", "feat/a api four"}, got)
	assert.Equal(t, []string{runlogs.CodeLinesDropped}, warned)
}

func TestAFollowTheDaemonRefusesReadsTheLogInstead(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "api", "api ready")
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{err: errors.New("unknown action lines")}}

	got, _, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}, Follow: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/a api api ready"}, got)
}

func TestAnInterruptedFollowEndsWithoutError(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: lines("api ready"), wait: true}}
	ctx, cancel := context.WithCancel(t.Context())

	err := w.usecase.Watch(ctx, runlogs.Request{Worktrees: []string{featA}, Follow: true}, runlogs.Out{
		Emit: func(runlogs.Entry) error { cancel(); return nil },
		Warn: func(domain.Warning) {},
	})

	assert.NoError(t, err, "a follow is stopped by an interrupt, exit 0")
}

// The review of #268: a quiet job, then Ctrl+C, exited 130 where an interrupted follow exits 0 (D3).
func TestAFollowInterruptedBeforeItsFirstLineEndsWithoutError(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	ctx, cancel := context.WithCancel(t.Context())
	w.daemon.lines[key(w.at(featA), "api")] = []script{{wait: true, before: cancel}}

	got, _, err := w.watched(ctx, t, runlogs.Request{Worktrees: []string{featA}, Job: "api", Follow: true})

	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestAReaderThatFailsEndsTheStreamWithItsError(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "api", "one", "two")
	gone := errors.New("broken pipe")
	emitted := 0

	err := w.usecase.Watch(t.Context(), runlogs.Request{Worktrees: []string{featA}}, runlogs.Out{
		Emit: func(runlogs.Entry) error { emitted++; return gone },
		Warn: func(domain.Warning) {},
	})

	assert.ErrorIs(t, err, gone)
	assert.Equal(t, 1, emitted, "no line is emitted once the reader failed")
}

// The review of #268: a worktree where nothing ran, and a job named that never ran, said nothing under --follow (B6, D6).
func TestWatchSaysWhatItHasNoLineOf(t *testing.T) {
	w := newWorld(t)
	var absent []runlogs.Absent
	out := runlogs.Out{
		Emit:   func(runlogs.Entry) error { return nil },
		Absent: func(a runlogs.Absent) error { absent = append(absent, a); return nil },
		Warn:   func(domain.Warning) {},
	}

	require.NoError(t, w.usecase.Watch(t.Context(), runlogs.Request{Worktrees: []string{featB}, Follow: true}, out))
	require.NoError(t, w.usecase.Watch(t.Context(), runlogs.Request{Worktrees: []string{featA}, Job: "never", Follow: true}, out))

	assert.Equal(t, []runlogs.Absent{{Worktree: featB}, {Worktree: featA, Job: "never"}}, absent)
}

// The review of #268: a log that cannot be read read as a job that never ran.
func TestALogThatCannotBeReadIsWarnedOnItsJob(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, os.MkdirAll(run.LogDir(w.stateDir, featA)+"/api.log", 0o700))

	read := w.read(t, runlogs.Request{Worktrees: []string{featA}})
	_, warned, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}})

	require.Equal(t, []string{"api"}, namesOf(read.Items[0]), "a log that cannot be read is a job that ran")
	assert.Equal(t, runlogs.JobStopped, jobOf(t, read.Items[0], "api").State)
	assert.Equal(t, []string{runlogs.CodeLogUnreadable}, codes(read.Items[0].Warnings))
	require.NoError(t, err)
	assert.Equal(t, []string{runlogs.CodeLogUnreadable}, warned)
}

// The review of #268: --job refused a job the daemon still runs that run.toml no longer declares.
func TestAJobTheDaemonRunsIsReadByNameEvenOnceRunTomlDropsIt(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "old", "still up")
	w.daemon.listed = []run.Job{ran("old", w.at(featA), run.Running)}

	read := w.read(t, runlogs.Request{Worktrees: []string{featA}, Job: "old"})
	failure := w.refused(t, runlogs.Request{Worktrees: []string{featB}, Job: "old"})

	assert.Equal(t, []string{"still up"}, textsOf(jobOf(t, read.Items[0], "old")))
	assert.Equal(t, []string{runlogs.CodeJobUnknown}, fieldCodes(failure.Fields), "a worktree the daemon runs no such job in")
}

// The review of #268: a shared service's log that cannot be read was warned once per worktree that does not show it.
func TestAnUnreadableLogIsWarnedOnlyWhereItsJobShows(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, os.MkdirAll(run.LogDir(w.stateDir, "main")+"/db.log", 0o700))

	_, warned, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{"main", featA, featB}})

	require.NoError(t, err)
	assert.Equal(t, []string{runlogs.CodeLogUnreadable}, warned, "main shows its shared service; feat/a and feat/b, which hold no claim, do not")
}

// The review of #268: the follow said a worktree where nothing ran after every other worktree's lines, not where the read says it.
func TestWatchSaysWhatNeverRanInTheReadsOrder(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "migrate", "migrated")
	var said []string
	out := runlogs.Out{
		Emit:   func(e runlogs.Entry) error { said = append(said, e.Worktree+" "+e.Text); return nil },
		Absent: func(a runlogs.Absent) error { said = append(said, a.Worktree+" nothing"); return nil },
		Warn:   func(domain.Warning) {},
	}

	require.NoError(t, w.usecase.Watch(t.Context(), runlogs.Request{Worktrees: []string{featB, featA}}, out))

	assert.Equal(t, []string{"feat/b nothing", "feat/a migrated"}, said)
}

func TestASearchReadsTheWholeLogItsBackupsFirst(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "api", "GET /b", "error twice", "GET /c")
	writeFile(t, run.LogDir(w.stateDir, featA)+"/api.log.1", "2026-11-03T20:00:00Z  GET /a\n2026-11-03T20:00:01Z  ERROR boom\n")

	for name, c := range map[string]struct {
		req   runlogs.Request
		texts []string
		n     []int
	}{
		"text":       {req: runlogs.Request{Search: "error"}, texts: []string{"ERROR boom", "error twice"}, n: []int{2, 4}},
		"match_case": {req: runlogs.Request{Search: "error", MatchCase: true}, texts: []string{"error twice"}, n: []int{4}},
		"regex":      {req: runlogs.Request{Search: "^GET /[ab]$", Regex: true}, texts: []string{"GET /a", "GET /b"}, n: []int{1, 3}},
		"since":      {req: runlogs.Request{Since: at.Add(time.Second).Format(time.RFC3339)}, texts: []string{"error twice", "GET /c"}, n: []int{4, 5}},
	} {
		t.Run(name, func(t *testing.T) {
			c.req.Worktrees, c.req.Job = []string{featA}, "api"
			api := jobOf(t, w.read(t, c.req).Items[0], "api")

			assert.Equal(t, c.texts, textsOf(api))
			var n []int
			for _, line := range api.Lines {
				n = append(n, line.N)
			}
			assert.Equal(t, c.n, n, "each line numbered in the whole log")
			assert.Equal(t, 5, api.Count, "the whole log, its backup included")
		})
	}
}

func TestASearchKeepsItsLastMatches(t *testing.T) {
	w := newWorld(t)
	texts := make([]string, run.MaxTail+2)
	for i := range texts {
		texts[i] = "hit"
	}
	w.logged(t, featA, "api", texts...)

	api := jobOf(t, w.read(t, runlogs.Request{Worktrees: []string{featA}, Job: "api", Search: "hit"}).Items[0], "api")

	assert.Len(t, api.Lines, run.MaxTail)
	assert.Equal(t, run.MaxTail+2, api.Lines[run.MaxTail-1].N, "the newest matches are kept")
}

func TestASearchThatIsNoRegularExpressionIsAProblemOfItsField(t *testing.T) {
	for name, c := range map[string]struct {
		req  runlogs.Request
		path string
		code string
	}{
		"regex": {req: runlogs.Request{Worktrees: []string{featA}, Search: "(", Regex: true}, path: runlogs.PathSearch, code: runlogs.CodeRegexInvalid},
		"since": {req: runlogs.Request{Worktrees: []string{featA}, Since: "14:00"}, path: runlogs.PathSince, code: runlogs.CodeSinceInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			failure := newWorld(t).refused(t, c.req)
			require.Len(t, failure.Fields, 1)
			assert.Equal(t, c.path, failure.Fields[0].Path)
			assert.Equal(t, c.code, failure.Fields[0].Code)
		})
	}
	read := newWorld(t).read(t, runlogs.Request{Worktrees: []string{featA}, Search: "("})
	assert.Equal(t, domain.StatusDone, read.Items[0].Status, "a search as text is never refused")
}

func TestADockerComposeStackSaysHowToFollowItsContainers(t *testing.T) {
	w := newWorld(t)
	w.logged(t, featA, "stack", "Container acme-feat-a-cache-1  Started")
	stack := ran("stack", w.at(featA), run.Running)
	stack.Hold = run.HoldDetached
	w.daemon.listed = []run.Job{stack, ran("api", w.at(featA), run.Running)}

	read := w.read(t, runlogs.Request{Worktrees: []string{featA}})

	assert.Regexp(t, `^docker compose -p \S+ logs -f$`, jobOf(t, read.Items[0], "stack").ComposeLogs, "its project, the worktree's")
	assert.Equal(t, "docker compose up -d", jobOf(t, read.Items[0], "stack").Command)
	assert.Empty(t, jobOf(t, read.Items[0], "api").ComposeLogs, "a job of wtm's logs to wtm")
}

func TestAFollowedSearchStreamsOnlyTheLinesItMatches(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: lines("GET / 200", "POST /x 500 error", "GET / 200")}}

	got, _, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}, Job: "api", Follow: true, Search: "error"})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/a api POST /x 500 error"}, got)
}

// The review of #270: a job that keeps no log printed every line the daemon keeps, matching or not.
func TestASearchSiftsTheDaemonsLinesOfAJobThatKeepsNoLog(t *testing.T) {
	w := newWorld(t)
	w.usecase.StateDir = ""
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: lines("GET / 200", "POST /x 500 error", "GET / 200")}}

	api := jobOf(t, w.read(t, runlogs.Request{Worktrees: []string{featA}, Job: "api", Search: "error"}).Items[0], "api")

	assert.Equal(t, []string{"POST /x 500 error"}, textsOf(api))
}

// The review of #270: a follow from a time streamed every line, the time dropped.
func TestAFollowFromATimeStreamsOnlyTheLinesPrintedSince(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: lines("old", "new", "newer")}}

	got, _, err := w.watched(t.Context(), t, runlogs.Request{Worktrees: []string{featA}, Job: "api", Follow: true, Since: at.Add(time.Second).Format(time.RFC3339)})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/a api new", "feat/a api newer"}, got)
}

// The review of #270: a running job that printed nothing left the web reading its lines.
func TestAFollowSaysItIsSettledBeforeAnyLine(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{}}
	var said []string

	err := w.usecase.Watch(t.Context(), runlogs.Request{Worktrees: []string{featA}, Job: "api", Follow: true}, runlogs.Out{
		Settled: func() { said = append(said, "settled") },
		Emit:    func(runlogs.Entry) error { said = append(said, "line"); return nil },
		Absent:  func(runlogs.Absent) error { return nil },
		Warn:    func(domain.Warning) {},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"settled"}, said)
	assert.Error(t, w.usecase.Watch(t.Context(), runlogs.Request{Worktrees: []string{"nope"}}, runlogs.Out{Settled: func() { t.Error("a refused request is never settled") }}))
}

// The review of #268: the job a follow followed to its end was dropped; the web needs it for a crash.
func TestAFollowSaysHowItsJobEnded(t *testing.T) {
	w := newWorld(t)
	w.daemon.listed = []run.Job{ran("api", w.at(featA), run.Running)}
	w.daemon.lines[key(w.at(featA), "api")] = []script{{lines: lines("boom"), ended: run.Job{State: run.Crashed, ExitCode: new(1)}}}
	var ended []runlogs.Ended

	err := w.usecase.Watch(t.Context(), runlogs.Request{Worktrees: []string{featA}, Job: "api", Follow: true}, runlogs.Out{
		Emit:   func(runlogs.Entry) error { return nil },
		Absent: func(runlogs.Absent) error { return nil },
		Ended:  func(e runlogs.Ended) error { ended = append(ended, e); return nil },
		Warn:   func(domain.Warning) {},
	})

	require.NoError(t, err)
	assert.Equal(t, []runlogs.Ended{{Worktree: featA, Job: "api", State: runlogs.JobCrashed, ExitCode: new(1)}}, ended)
}
