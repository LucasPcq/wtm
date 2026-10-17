package operation

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

func TestBeginRecordsTheOperationAndTakesItsLocks(t *testing.T) {
	l := &locker{hold: &hold{}}

	op, err := service(l, &publisher{}).Begin("create", []domain.Lock{repoLock}, domain.RunOptions{Surface: "cli", CorrelationID: "corr-1"})

	require.NoError(t, err)
	assert.Equal(t, "op-1", op.ID)
	want := domain.Operation{ID: "op-1", PID: os.Getpid(), Command: "create", Surface: "cli", CorrelationID: "corr-1", StartedAt: startedAt, Locks: []domain.Lock{repoLock}}
	assert.Equal(t, []domain.Operation{want}, l.taken)
}

func TestBeginRefusesAnInvalidCorrelationIDBeforeTakingAnyLock(t *testing.T) {
	l := &locker{hold: &hold{}}

	op, err := service(l, &publisher{}).Begin("create", []domain.Lock{repoLock}, domain.RunOptions{CorrelationID: "bad\x01id"})

	found, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "Begin = %v, want a *domain.Error", err)
	assert.Equal(t, domain.KindInvalid, found.Kind)
	assert.Equal(t, "the correlation id contains a control character", found.Message)
	require.NotNil(t, op)
	assert.Equal(t, "op-1", op.ID)
	assert.Empty(t, l.taken)
	op.End()
}

func TestBeginOnADryRunTakesNoLock(t *testing.T) {
	l := &locker{hold: &hold{}}

	op, err := service(l, &publisher{}).Begin("create", []domain.Lock{repoLock}, domain.RunOptions{DryRun: true})

	require.NoError(t, err)
	assert.Equal(t, "op-1", op.ID)
	assert.Empty(t, l.taken)
}

func TestBeginReturnsTheOpWithTheLockersError(t *testing.T) {
	held := &domain.Error{Kind: domain.KindConflict, Code: "lock.held"}

	op, err := service(&locker{err: held}, &publisher{}).Begin("create", []domain.Lock{repoLock}, domain.RunOptions{})

	assert.ErrorIs(t, err, held)
	require.NotNil(t, op)
	assert.Equal(t, "op-1", op.ID)
	op.End()
}

func TestPlanUnderLocksPlansOnceOnADryRun(t *testing.T) {
	op := begun(t, &hold{}, domain.RunOptions{DryRun: true})
	plans := 0

	got, _, err := PlanUnderLocks(t.Context(), op, func(context.Context) (plan, []domain.Warning, error) {
		plans++
		return plan{locks: []domain.Lock{repoLock, featureLock}}, nil, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, plans)
	assert.Equal(t, []domain.Lock{repoLock, featureLock}, got.locks)
}

func TestPlanUnderLocksPlansAgainOnlyWhenItTookANewLock(t *testing.T) {
	h := &hold{held: []domain.Lock{repoLock}}
	op := begun(t, h, domain.RunOptions{})
	plans := 0

	_, _, err := PlanUnderLocks(t.Context(), op, func(context.Context) (plan, []domain.Warning, error) {
		plans++
		return plan{locks: []domain.Lock{repoLock, featureLock}}, nil, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, plans, "the first plan named featureLock, not held yet; the second named nothing new")
	assert.Equal(t, []domain.Lock{repoLock, featureLock}, h.held)
}

func TestPlanUnderLocksReturnsThePlannersError(t *testing.T) {
	failed := errors.New("observe failed")

	_, _, err := PlanUnderLocks(t.Context(), begun(t, &hold{}, domain.RunOptions{}), func(context.Context) (plan, []domain.Warning, error) {
		return plan{}, nil, failed
	})

	assert.ErrorIs(t, err, failed)
}

func TestFingerprintIsStableForEqualValuesAndDiffersForDifferentOnes(t *testing.T) {
	first, err := Fingerprint(map[string]string{"branch": "feat/a"})
	require.NoError(t, err)
	again, err := Fingerprint(map[string]string{"branch": "feat/a"})
	require.NoError(t, err)
	other, err := Fingerprint(map[string]string{"branch": "feat/b"})
	require.NoError(t, err)

	assert.Equal(t, first, again)
	assert.NotEqual(t, first, other)
	assert.Len(t, first, 2*fingerprintBytes)
}

func TestFingerprintFailsOnAValueJSONCannotEncode(t *testing.T) {
	_, err := Fingerprint(make(chan int))

	assert.Error(t, err)
}

func TestExpectRefusesADifferentFingerprintAsStale(t *testing.T) {
	err := begun(t, &hold{}, domain.RunOptions{ExpectFingerprint: "aaa"}).Expect("bbb")

	found, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "Expect(%q) = %v, want a *domain.Error", "bbb", err)
	assert.Equal(t, domain.KindPrecondition, found.Kind)
	assert.Equal(t, domain.CodePlanStale, found.Code)
	assert.Equal(t, domain.Params{"expected": "aaa", "found": "bbb"}, found.Params)
	assert.True(t, domain.PlanStale(err))
}

func TestExpectAcceptsTheSameFingerprintOrNone(t *testing.T) {
	assert.NoError(t, begun(t, &hold{}, domain.RunOptions{ExpectFingerprint: "aaa"}).Expect("aaa"))
	assert.NoError(t, begun(t, &hold{}, domain.RunOptions{}).Expect("bbb"))
}

func TestStartedAndFinishedReportTheEventsInOrder(t *testing.T) {
	p := &publisher{}
	var surfaced []domain.Event
	op, err := service(&locker{hold: &hold{}}, p).Begin("create", nil, domain.RunOptions{Surface: "cli", Report: func(e domain.Event) { surfaced = append(surfaced, e) }})
	require.NoError(t, err)

	report := op.Started()
	report(domain.UnitStarted{Subject: "feat/a"})
	op.Finished(nil)

	want := []domain.Event{
		domain.OperationStarted{Command: "create", Surface: "cli"},
		domain.UnitStarted{Subject: "feat/a"},
		domain.OperationFinished{Command: "create", Surface: "cli", OK: true},
	}
	assert.Equal(t, want, p.published)
	assert.Equal(t, want, surfaced)
	assert.Len(t, p.opened, 1)
}

func TestFinishedCarriesTheFailuresCode(t *testing.T) {
	p := &publisher{}
	op, err := service(&locker{hold: &hold{}}, p).Begin("create", nil, domain.RunOptions{Surface: "cli"})
	require.NoError(t, err)

	op.Started()
	op.Finished(&domain.Error{Kind: domain.KindInternal, Code: "unit.step_failed"})

	assert.Equal(t, domain.OperationFinished{Command: "create", Surface: "cli", Code: "unit.step_failed"}, p.published[len(p.published)-1])
}

func TestWarningsAddsLocksUnavailableWhenUnprotectedAndKeepsEachOnce(t *testing.T) {
	dirty := domain.Warning{Code: "create.ff_dirty", Message: "dirty", Params: domain.Params{"branch": "main"}}

	got := begun(t, &hold{unprotected: true}, domain.RunOptions{}).Warnings([]domain.Warning{dirty, dirty})

	assert.Equal(t, []domain.Warning{dirty, domain.LocksUnavailable}, got)
}

func TestWarningsAddsNothingWhenProtected(t *testing.T) {
	dirty := domain.Warning{Code: "create.ff_dirty", Message: "dirty"}

	assert.Equal(t, []domain.Warning{dirty}, begun(t, &hold{}, domain.RunOptions{}).Warnings([]domain.Warning{dirty}))
}

func TestEndReleasesTheLocks(t *testing.T) {
	h := &hold{}

	begun(t, h, domain.RunOptions{}).End()

	assert.True(t, h.released)
}

func TestEndIsSafeOnAnOpThatHoldsNone(t *testing.T) {
	h := &hold{}

	begun(t, h, domain.RunOptions{DryRun: true}).End()

	assert.False(t, h.released)
}
