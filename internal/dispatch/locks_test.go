package dispatch_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain/operation"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

func TestAWorktreeHeldByAnotherOperationIsA409NamingItsHolder(t *testing.T) {
	b := bench(t)
	first := &repo{}
	entered, release := first.held()
	done := make(chan dispatchtest.Ran[plan, detail])
	go func() {
		done <- run(t.Context(), runParams{Bench: b, Repo: first, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{Surface: dispatch.SurfaceTUI}})
	}()
	<-entered

	second := &repo{}
	ran := run(t.Context(), runParams{Bench: b, Repo: second, Request: request{Branches: []string{"feat/b", "feat/a"}}})
	release()

	failure := problem(t, ran.Err)
	assert.Equal(t, kernel.KindConflict, failure.Kind())
	assert.Equal(t, kernel.CodeLockHeld, failure.Base().Code)
	assert.Equal(t, kernel.Params{
		kernel.ParamKey:       "wt:feat/a",
		kernel.ParamOp:        "op-1",
		kernel.ParamCommand:   commandName,
		kernel.ParamPID:       strconv.Itoa(os.Getpid()),
		kernel.ParamSurface:   string(dispatch.SurfaceTUI),
		kernel.ParamStartedAt: dispatchtest.Clock.Format(time.RFC3339),
	}, failure.Base().Params)
	assert.Empty(t, second.listed(), "the refused run applied")
	require.NoError(t, (<-done).Err)
}

func TestOperationsOnOtherWorktreesRunSideBySide(t *testing.T) {
	b := bench(t)
	first := &repo{}
	entered, release := first.held()
	done := make(chan dispatchtest.Ran[plan, detail])
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
	query := operationsQuery(b)
	r := &repo{}
	entered, release := r.held()
	done := make(chan dispatchtest.Ran[plan, detail])
	go func() {
		done <- run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{Surface: dispatch.SurfaceCLI, CorrelationID: "agent-7"}})
	}()
	<-entered

	during, err := query.Run(t.Context(), operations.Request{})
	release()
	require.NoError(t, err)
	require.NoError(t, (<-done).Err)
	after, err := query.Run(t.Context(), operations.Request{})
	require.NoError(t, err)

	assert.Equal(t, []operation.Operation{{
		ID:            "op-1",
		PID:           os.Getpid(),
		Command:       commandName,
		Surface:       string(dispatch.SurfaceCLI),
		CorrelationID: "agent-7",
		StartedAt:     dispatchtest.Clock,
		Locks: []operation.Lock{
			{Scope: string(kernel.LockRepo), Mode: string(kernel.LockShared)},
			{Scope: string(kernel.LockWorktree), Name: "feat/a", Mode: string(kernel.LockExclusive)},
		},
	}}, during.Operations)
	assert.Empty(t, after.Operations)
}

func TestADryRunTakesNoLock(t *testing.T) {
	b := bench(t)
	release := holding(t, b, "feat/a")
	defer release()

	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{DryRun: true}})

	require.NoError(t, ran.Err, "a preview is never refused for a lock: the fingerprint guards the run that applies it")
	running, err := b.Dispatcher.Operations(t.Context())
	require.NoError(t, err)
	assert.Len(t, running, 1, "a dry run leaves no holder")
}

func TestTheLocksADefaultNamesAreTakenBeforeApply(t *testing.T) {
	b := bench(t)
	release := holding(t, b, "feat/a")
	defer release()
	r := &repo{base: "feat/a"}

	ran := run(t.Context(), runParams{Bench: b, Repo: r, Request: request{}})

	failure := problem(t, ran.Err)
	assert.Equal(t, kernel.CodeLockHeld, failure.Base().Code)
	assert.Equal(t, "wt:feat/a", failure.Base().Params[kernel.ParamKey])
	assert.Empty(t, r.listed(), "the branch a default named was created under no lock")
}

func TestTakingMoreLocksReadsTheFactsAgain(t *testing.T) {
	given := &repo{base: "feat/a"}
	require.NoError(t, run(t.Context(), runParams{Bench: bench(t), Repo: given, Request: request{Branches: []string{"feat/a"}}}).Err)
	assert.Equal(t, 1, given.observations(), "the request named every lock it needed")

	defaulted := &repo{base: "feat/a"}
	require.NoError(t, run(t.Context(), runParams{Bench: bench(t), Repo: defaulted, Request: request{}}).Err)
	assert.Equal(t, 2, defaulted.observations(), "the facts read before the lock was taken may be stale")
	assert.Equal(t, []string{"feat/a"}, defaulted.listed())
}

func TestTheHolderListsTheLocksTheDefaultsAdded(t *testing.T) {
	b := bench(t)
	r := &repo{base: "feat/a"}
	entered, release := r.held()
	done := make(chan dispatchtest.Ran[plan, detail])
	go func() { done <- run(t.Context(), runParams{Bench: b, Repo: r, Request: request{}}) }()
	<-entered

	running, err := b.Dispatcher.Operations(t.Context())
	release()
	require.NoError(t, (<-done).Err)
	require.NoError(t, err)
	require.Len(t, running, 1)
	assert.Equal(t, []operation.Lock{
		{Scope: string(kernel.LockRepo), Mode: string(kernel.LockShared)},
		{Scope: string(kernel.LockWorktree), Name: "feat/a", Mode: string(kernel.LockExclusive)},
	}, running[0].Locks)
}

func TestAWorktreeLockWithNoNameIsRefused(t *testing.T) {
	r := &repo{lockOf: func(request) []kernel.LockKey {
		return []kernel.LockKey{{Scope: kernel.LockWorktree, Mode: kernel.LockExclusive}}
	}}

	ran := run(t.Context(), runParams{Bench: bench(t), Repo: r, Request: request{Branches: []string{"feat/a"}}})

	assert.Equal(t, kernel.KindInternal, problem(t, ran.Err).Kind())
	assert.Empty(t, r.listed())
}
