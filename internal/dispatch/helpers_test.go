package dispatch_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

// The few helpers the tests share. Everything a test file holds is a test.

const (
	envStateDir = "WTM_TEST_DISPATCH_STATE"
	envBranches = "WTM_TEST_DISPATCH_BRANCHES"
	readyLine   = "held"
)

func bench(t testing.TB) *dispatchtest.Bench {
	t.Helper()
	return dispatchtest.New(t, dispatchtest.BenchParams{})
}

type runParams struct {
	Bench   *dispatchtest.Bench
	Repo    *repo
	Request request
	Options dispatch.Options
}

func run(ctx context.Context, params runParams) dispatchtest.Ran[plan, detail] {
	return dispatchtest.Run(ctx, dispatchtest.RunParams[request, facts, plan, detail]{
		Bench:   params.Bench,
		Command: createCommand(params.Repo),
		Events:  createEvents(),
		Request: params.Request,
		Options: params.Options,
	})
}

func handle(b *dispatchtest.Bench, r *repo) *dispatch.Handle[request, facts, plan, detail] {
	return dispatch.Register(dispatch.RegisterParams[request, facts, plan, detail]{Dispatcher: b.Dispatcher, Command: createCommand(r), Events: createEvents()})
}

func operationsQuery(b *dispatchtest.Bench) *dispatch.QueryHandle[operations.Request, operations.View] {
	return dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{Query: operations.Query(b.Dispatcher.Operations)})
}

// registry holds the example command and the operations query.
func registry(t *testing.T, r *repo) dispatch.Registry {
	t.Helper()
	b := bench(t)
	return dispatch.Registry{handle(b, r), operationsQuery(b)}
}

// problem is the kernel error a run failed with.
func problem(t testing.TB, err error) kernel.Error {
	t.Helper()
	var failure kernel.Error
	require.ErrorAs(t, err, &failure)
	return failure
}

// TestHelperProcess is the body of the process the cross-process test starts:
// it creates the branches it is given through dispatch, on the state dir it
// is pointed at, says so, and holds their locks until its stdin closes. A
// read keeps it waiting where a blocked channel alone would be taken for a
// deadlock, and the process killed.
func TestHelperProcess(t *testing.T) {
	stateDir := os.Getenv(envStateDir)
	if stateDir == "" {
		t.Skip("run by TestLocksAreTakenWholeAcrossProcesses")
	}
	r := &repo{}
	entered, release := r.held()
	go func() {
		<-entered
		fmt.Println(readyLine)
		_, _ = io.Copy(io.Discard, os.Stdin)
		release()
	}()
	ran := run(t.Context(), runParams{
		Bench:   dispatchtest.New(t, dispatchtest.BenchParams{StateDir: stateDir, OpPrefix: "child-"}),
		Repo:    r,
		Request: request{Branches: strings.Split(os.Getenv(envBranches), ",")},
	})
	require.NoError(t, ran.Err)
}

// fillable is the example's entry as a form fills it.
func fillable(t testing.TB, h *dispatch.Handle[request, facts, plan, detail]) dispatch.Fillable {
	t.Helper()
	f, ok := h.Fillable()
	require.True(t, ok)
	return f
}

func kindOf(progress kernel.Progress) string {
	switch progress.(type) {
	case *kernel.UnitStarted:
		return "unit.started"
	case *kernel.UnitFinished:
		return "unit.finished"
	case *kernel.PhaseStarted:
		return "phase.started"
	case *kernel.PhaseFinished:
		return "phase.finished"
	case *kernel.Output:
		return "output"
	case *kernel.StatusChanged:
		return "status"
	}
	return ""
}

// holding runs create on branches through b and keeps it in Apply, its locks
// held, until the release it returns.
func holding(t *testing.T, b *dispatchtest.Bench, branches ...string) (release func()) {
	t.Helper()
	r := &repo{}
	entered, let := r.held()
	done := make(chan dispatchtest.Ran[plan, detail], 1)
	go func() { done <- run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: branches}}) }()
	<-entered
	return func() {
		let()
		require.NoError(t, (<-done).Err)
	}
}
