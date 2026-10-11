package dispatch_test

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

func TestAWorktreeHeldByAnotherOperationIsA409NamingItsHolder(t *testing.T) {
	b := bench(t)
	first := &repo{}
	entered, release := first.held()
	done := make(chan dispatchtest.Ran[plan, string])
	go func() {
		done <- run(t.Context(), runParams{Bench: b, Repo: first, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{Surface: kernel.SurfaceTUI}})
	}()
	<-entered

	second := &repo{}
	ran := run(t.Context(), runParams{Bench: b, Repo: second, Request: request{Branches: []string{"feat/b", "feat/a"}}})
	release()

	failure := problem(t, ran.Err)
	assert.Equal(t, kernel.KindConflict, failure.Kind())
	assert.Equal(t, dispatch.CodeLockHeld, failure.Base().Code)
	assert.Equal(t, kernel.Params{
		dispatch.ParamKey:       "wt:feat/a",
		dispatch.ParamOp:        "op-1",
		dispatch.ParamCommand:   commandName,
		dispatch.ParamPID:       strconv.Itoa(os.Getpid()),
		dispatch.ParamSurface:   string(kernel.SurfaceTUI),
		dispatch.ParamStartedAt: dispatchtest.Clock.Format(time.RFC3339),
	}, failure.Base().Params)
	assert.Empty(t, second.listed(), "the refused run applied")
	require.NoError(t, (<-done).Err)
}

func TestOperationsOnOtherWorktreesRunSideBySide(t *testing.T) {
	b := bench(t)
	first := &repo{}
	entered, release := first.held()
	done := make(chan dispatchtest.Ran[plan, string])
	go func() {
		done <- run(t.Context(), runParams{Bench: b, Repo: first, Request: request{Branches: []string{"feat/a"}}})
	}()
	<-entered

	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/b"}}})
	release()

	assert.NoError(t, ran.Err)
	require.NoError(t, (<-done).Err)
}

func TestTheOperationsInProgressAreReadFromTheirHolders(t *testing.T) {
	b := bench(t)
	query := dispatch.RegisterQuery(operations.Query(b.Dispatcher.Operations))
	r := &repo{}
	entered, release := r.held()
	done := make(chan dispatchtest.Ran[plan, string])
	go func() {
		done <- run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{Surface: kernel.SurfaceCLI, CorrelationID: "agent-7"}})
	}()
	<-entered

	during, err := query.Run(t.Context(), operations.Request{})
	release()
	require.NoError(t, err)
	require.NoError(t, (<-done).Err)
	after, err := query.Run(t.Context(), operations.Request{})
	require.NoError(t, err)

	assert.Equal(t, []kernel.Operation{{
		ID:            "op-1",
		PID:           os.Getpid(),
		Command:       commandName,
		Surface:       kernel.SurfaceCLI,
		CorrelationID: "agent-7",
		StartedAt:     dispatchtest.Clock,
		Locks: []kernel.LockKey{
			{Scope: kernel.LockRepo, Mode: kernel.LockShared},
			{Scope: kernel.LockWorktree, Name: "feat/a", Mode: kernel.LockExclusive},
		},
	}}, during.Operations)
	assert.Empty(t, after.Operations)
}

// — Across processes: all or nothing, and a dead process holds nothing —

func TestLocksAreTakenWholeAcrossProcesses(t *testing.T) {
	b := bench(t)
	child := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	child.Env = append(os.Environ(), envStateDir+"="+b.StateDir, envBranches+"=feat/b")
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = stdin.Close(); _ = child.Process.Kill(); _ = child.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, readyLine+"\n", line)

	refused := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a", "feat/b"}}})
	failure := problem(t, refused.Err)
	assert.Equal(t, dispatch.CodeLockHeld, failure.Base().Code)
	assert.Equal(t, strconv.Itoa(child.Process.Pid), failure.Base().Params[dispatch.ParamPID])

	alone := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}})
	assert.NoError(t, alone.Err, "the refused run kept feat/a")

	require.NoError(t, child.Process.Kill())
	_ = child.Wait()
	after := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/b"}}})
	assert.NoError(t, after.Err, "a dead process still held feat/b")
}
