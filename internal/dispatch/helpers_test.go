package dispatch_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

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

func registry(t *testing.T, r *repo) dispatch.Registry {
	t.Helper()
	b := bench(t)
	return dispatch.Registry{handle(b, r), operationsQuery(b)}
}

func problem(t testing.TB, err error) kernel.Error {
	t.Helper()
	var failure kernel.Error
	require.ErrorAs(t, err, &failure)
	return failure
}

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
