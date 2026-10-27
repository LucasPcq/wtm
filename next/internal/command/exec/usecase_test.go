package exec_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failsInB fails, saying so on stderr, in feat/b's worktree only.
const failsInB = `test "$(basename "$PWD")" != wt-feat-b || { echo boom >&2; exit 3; }; echo ok`

func TestTheCommandRunsInEachTargetFromItsRootAsGitSpellsIt(t *testing.T) {
	w := newWorld(t)

	result, report := w.apply(t, running("pwd", featA, featB))

	require.Len(t, result.Items, 2)
	for i, name := range []string{featA, featB} {
		item := result.Items[i]
		assert.Equal(t, name, item.Subject, "the items keep the plan's order")
		assert.Equal(t, domain.StatusDone, item.Status)
		require.NotNil(t, item.Detail)
		assert.Equal(t, w.at(name), item.Detail.Path)
		assert.Equal(t, []string{w.at(name)}, item.Detail.Tail, "the path the command runs in is the path the report names")
		assert.Equal(t, 0, *item.Detail.ExitCode)
	}
	assert.Empty(t, result.FollowUps)
	started := eventsOf[domain.PhaseStarted](report.All())
	require.Len(t, started, 2)
	assert.Equal(t, domain.PhaseExec, started[0].Phase)
	assert.Equal(t, result.Items[0].Detail.Log, logOf(started, featA))
	assert.Contains(t, eventsOf[domain.Output](report.All()), domain.Output{Subject: featB, Line: w.at(featB)})
	assert.Len(t, eventsOf[domain.UnitFinished](report.All()), 2)
}

func TestACommandThatFailsInOneTargetFailsItAloneAndIsOfferedAgainThere(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, running(failsInB, featA, featB))

	assert.Equal(t, map[string]domain.Status{featA: domain.StatusDone, featB: domain.StatusFailed}, statuses(result.Items))
	failed := itemOf(t, result.Items, featB)
	assert.Equal(t, exec.CodeFailed, failed.Err.Code)
	assert.Equal(t, "exited 3", failed.Err.Message)
	assert.Equal(t, "3", failed.Err.Params["exit_code"])
	assert.Equal(t, 3, *failed.Detail.ExitCode)
	assert.Equal(t, []string{"boom"}, failed.Detail.Tail)
	require.Len(t, result.FollowUps, 1)
	retry := result.FollowUps[0]
	assert.Equal(t, exec.CodeRetry, retry.Code)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{featB}}, {Path: exec.PathCommand, Values: []string{failsInB}}}, retry.Answers)
	assert.Equal(t, &retry, failed.Err.FollowUp, "a quiet run names where to run it again")
}

func TestEachRunHasALogOfItsOwnTheNextRunRemovingTheEndedOnes(t *testing.T) {
	w := newWorld(t)

	first, _ := w.apply(t, running("echo first", featA))
	second, _ := w.apply(t, running("echo second", featA))

	assert.NoFileExists(t, first.Items[0].Detail.Log, "an ended run's log goes with the next run")
	assert.Equal(t, w.logDir(featA), filepath.Dir(second.Items[0].Detail.Log))
	written, err := os.ReadFile(second.Items[0].Detail.Log)
	require.NoError(t, err)
	assert.Equal(t, "second\n", string(written))
}

func TestARunNeverRemovesNorWritesTheLogOfARunStillGoing(t *testing.T) {
	w := newWorld(t)
	going, log, err := shell.OpenRunLog(w.logDir(featA), "op-going")
	require.NoError(t, err)
	_, _ = io.WriteString(log, "still going\n")

	result, _ := w.apply(t, running("echo mine", featA))
	require.NoError(t, log.Close())

	written, err := os.ReadFile(going)
	require.NoError(t, err)
	assert.Equal(t, "still going\n", string(written))
	assert.NotEqual(t, going, result.Items[0].Detail.Log)
	assert.Len(t, w.logsOf(t, featA), 2)
}

func TestEachLogOfARunStaysUntilTheWholeRunEnds(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, exec.Request{Worktrees: []string{featA, featB}, Command: "true", Jobs: "1"})
	planned.Op = "op-first"
	report := &operationtest.Recorder{}
	report.On = func(event domain.Event) {
		if started, ok := event.(domain.PhaseStarted); ok && started.Subject == featB {
			// another run in feat/a, once feat/a is done here and feat/b still running
			_, log, err := shell.OpenRunLog(w.logDir(featA), "op-other")
			require.NoError(t, err)
			require.NoError(t, log.Close())
		}
	}

	result := w.usecase.Apply(t.Context(), planned, report.Add)

	assert.FileExists(t, result.Items[0].Detail.Log, "a log is removed only once its whole run ended")
}

func TestACRLFLineKeepsItsTextInTheTailAndTheLog(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, exec.Request{Worktrees: []string{featA}, Command: `printf 'ok\r\n'`, Print: true})

	detail := result.Items[0].Detail
	assert.Equal(t, []string{"ok"}, detail.Tail)
	assert.Equal(t, "ok\n", detail.Output)
	written, err := os.ReadFile(detail.Log)
	require.NoError(t, err)
	assert.Equal(t, "ok\n", string(written))
}

func TestTheLogsOfAWorktreeOnNoBranchAreItsOwn(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, running("true", w.at("detached")))

	assert.Equal(t, w.logDir(w.at("detached")), filepath.Dir(result.Items[0].Detail.Log))
}

func TestALogThatCannotBeOpenedIsNamedNowhereAndTheCommandRuns(t *testing.T) {
	w := newWorld(t)
	w.usecase.OpenLog = func(string, string) (string, io.WriteCloser, error) { return "", nil, errNoLog }

	result, report := w.apply(t, running("echo ran", featA))

	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Empty(t, result.Items[0].Detail.Log)
	assert.Empty(t, logOf(eventsOf[domain.PhaseStarted](report.All()), featA))
	assert.Equal(t, []string{"ran"}, result.Items[0].Detail.Tail)
}

func TestTheCallersPortsAndWtmVariablesNeverReachATarget(t *testing.T) {
	w := newWorld(t)
	t.Setenv("DB_PORT", "9999")
	t.Setenv("WTM_BRANCH", "elsewhere")

	result, _ := w.apply(t, running(`echo "${DB_PORT-unset} ${WTM_BRANCH-unset}"`, featA))

	assert.Equal(t, []string{"unset unset"}, result.Items[0].Detail.Tail)
}

func TestANumberedIsolatedWorktreeRunsWithTheEnvironmentItsHooksGet(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, running(`echo "$WTM_BRANCH $DB_PORT"`, numbered, unnumbered))

	assert.Equal(t, []string{numbered + " 5452"}, itemOf(t, result.Items, numbered).Detail.Tail)
	assert.Equal(t, []string{" "}, itemOf(t, result.Items, unnumbered).Detail.Tail, "no ordinal, no run environment")
}

func TestPrintKeepsTheWholeOutputAndTheTailItsLastLines(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, exec.Request{Worktrees: []string{featA}, Command: "seq 25; printf 'old\\rnew \\033[31mred\\033[0m\\n'", Print: true})

	detail := result.Items[0].Detail
	assert.Len(t, detail.Tail, 20)
	assert.Equal(t, "7", detail.Tail[0])
	assert.Equal(t, "new red", detail.Tail[19], "the tail is as a terminal left it")
	assert.Equal(t, 26, len(splitLines(detail.Output)))
}

func TestEachTailLineIsWhatATerminalWouldShow(t *testing.T) {
	w := newWorld(t)
	lines := []string{`\033[1;31mred\033[0m`, `\033]8;;https://wtm.dev\033\\link\033]8;;\007`, `10%%\r50%%\r100%% done`, `\0337saved\0338`}

	result, _ := w.apply(t, running("printf '"+strings.Join(lines, `\n`)+`\n'`, featA))

	assert.Equal(t, []string{"red", "link", "100% done", "saved"}, result.Items[0].Detail.Tail)
}

func TestACancelStopsTheCommandsRunningAndReachesNoOther(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, exec.Request{Worktrees: []string{featA, featB}, Command: "sleep 5", Jobs: "1"})
	ctx, cancel := context.WithCancel(t.Context())
	report := newCancellingRecorder(cancel)

	result := w.usecase.Apply(ctx, planned, report.Add)

	assert.Equal(t, domain.StatusCancelled, result.Items[0].Status)
	assert.Equal(t, domain.ReasonInterrupted, result.Items[0].Reason)
	assert.Nil(t, result.Items[0].Detail.ExitCode, "stopped by a signal, it never exited")
	assert.Equal(t, domain.StatusSkipped, result.Items[1].Status)
	assert.Equal(t, domain.ReasonNotReached, result.Items[1].Reason)
	require.Len(t, result.FollowUps, 1)
	assert.Equal(t, []string{featA, featB}, result.FollowUps[0].Answers[0].Values)
}

func TestARunIsAnOperationWhoseLocksAreShared(t *testing.T) {
	w := newWorld(t)
	held, err := w.usecase.Operations.Locker.Take(domain.Operation{ID: "other", Command: exec.Name, Surface: domain.SurfaceCLI, Locks: []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockShared}, {Scope: domain.ScopeWorktree, Name: featA, Mode: domain.LockShared}}})
	require.NoError(t, err)
	defer held.Release()

	outcome, err := w.usecase.Run(t.Context(), running("true", featA), domain.RunOptions{Surface: domain.SurfaceCLI})

	require.NoError(t, err, "two runs share a worktree")
	assert.True(t, outcome.Applied())
}

func TestARemovalHoldingATargetRefusesTheRunNamingItsHolder(t *testing.T) {
	w := newWorld(t)
	held, err := w.usecase.Operations.Locker.Take(domain.Operation{ID: "other", PID: os.Getpid(), Command: "clean", Surface: domain.SurfaceWeb, Locks: domain.WorktreeLocks([]string{featA})})
	require.NoError(t, err)
	defer held.Release()

	_, err = w.usecase.Run(t.Context(), running("true", featA), domain.RunOptions{Surface: domain.SurfaceCLI})

	failure := failureOf(t, err)
	assert.Equal(t, domain.KindConflict, failure.Kind)
	assert.Equal(t, featA+" is locked by clean, started from web (pid "+strconv.Itoa(os.Getpid())+")", failure.Message)
}
