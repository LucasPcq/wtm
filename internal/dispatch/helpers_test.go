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

func run(ctx context.Context, params runParams) dispatchtest.Ran[plan, string] {
	return dispatchtest.Run(ctx, dispatchtest.RunParams[request, facts, plan, string]{
		Bench:   params.Bench,
		Command: createCommand(params.Repo),
		Request: params.Request,
		Options: params.Options,
	})
}

func handle(b *dispatchtest.Bench, r *repo) *dispatch.Handle[request, facts, plan, string] {
	return dispatch.Register(dispatch.RegisterParams[request, facts, plan, string]{Dispatcher: b.Dispatcher, Command: createCommand(r)})
}

// registry holds the example command and the operations query.
func registry(t *testing.T, r *repo) dispatch.Registry {
	t.Helper()
	b := bench(t)
	return dispatch.Registry{handle(b, r), dispatch.RegisterQuery(operations.Query(b.Dispatcher.Operations))}
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
