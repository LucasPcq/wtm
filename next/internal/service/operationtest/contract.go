// Package operationtest is the contract every mutating use case's Run keeps with its operation (a dry run takes no lock and publishes nothing, a stale plan is refused before any event, the events of an apply arrive in order, each unit started finished, and the operation records the correlation ID of its own options), and the in-memory Locker, Publisher and event recorder of an operations service the commands' tests share.
package operationtest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Subject is one command's Run over a repository made for one check of the contract.
type Subject struct {
	// Run runs a request that applies a change with events, returning the operation it ran under and the fingerprint of its plan.
	Run func(ctx context.Context, opts domain.RunOptions) (Ran, error)
	// Stale changes the facts the request's plan depends on, so that a preview made before no longer holds.
	Stale func(t *testing.T)
	// Fail makes the apply of the request fail once it started, as a failing hook does.
	Fail func(t *testing.T)
}

// Ran is what Run returns of the outcome: its operation's id and its plan's fingerprint.
type Ran struct {
	Op          string
	Fingerprint string
}

// Contract checks the contract on the Subject that subject returns, a new one for each check, wired to the operations service it is given.
func Contract(t *testing.T, subject func(t *testing.T, ops operation.Service) Subject) {
	t.Run("dry_run_no_lock_no_event", func(t *testing.T) {
		rec := &Locks{}
		run := subject(t, service(rec)).Run
		reported := &Recorder{}

		ran, err := run(t.Context(), domain.RunOptions{DryRun: true, Report: reported.Add})

		require.NoError(t, err)
		assert.NotEmpty(t, ran.Fingerprint)
		assert.Empty(t, rec.Taken())
		assert.Empty(t, rec.Published.All())
		assert.Empty(t, reported.All())
	})

	t.Run("stale_plan_refused_before_events", func(t *testing.T) {
		rec := &Locks{}
		s := subject(t, service(rec))
		preview, err := s.Run(t.Context(), domain.RunOptions{DryRun: true})
		require.NoError(t, err)
		s.Stale(t)
		reported := &Recorder{}

		_, err = s.Run(t.Context(), domain.RunOptions{ExpectFingerprint: preview.Fingerprint, Report: reported.Add})

		assert.True(t, domain.PlanStale(err), "Run with a stale fingerprint = %v, want %s", err, domain.CodePlanStale)
		assert.Empty(t, rec.Published.All())
		assert.Empty(t, reported.All())
		assert.True(t, rec.Released(), "a lock is still held")
	})

	t.Run("events_in_order", func(t *testing.T) {
		rec := &Locks{}
		run := subject(t, service(rec)).Run
		reported := &Recorder{}

		_, err := run(t.Context(), domain.RunOptions{Surface: "cli", Report: reported.Add})

		require.NoError(t, err)
		inOrder(t, rec, reported.All(), true)
	})

	t.Run("correlation_id_recorded", func(t *testing.T) {
		rec := &Locks{}
		run := subject(t, service(rec)).Run

		_, err := run(t.Context(), domain.RunOptions{CorrelationID: "corr-1"})

		require.NoError(t, err)
		require.NotEmpty(t, rec.Taken(), "the run took no lock")
		require.NotEmpty(t, rec.Opened(), "the run published no event")
		for _, op := range append(rec.Taken(), rec.Opened()...) {
			assert.Equal(t, "corr-1", op.CorrelationID, "operation %s", op.ID)
		}
	})

	t.Run("invalid_correlation_id_refused", func(t *testing.T) {
		rec := &Locks{}
		run := subject(t, service(rec)).Run
		reported := &Recorder{}

		ran, err := run(t.Context(), domain.RunOptions{CorrelationID: "bad\x01id", Report: reported.Add})

		refused, ok := errors.AsType[*domain.Error](err)
		require.True(t, ok, "Run with an invalid correlation ID = %v, want a *domain.Error", err)
		assert.Equal(t, domain.KindInvalid, refused.Kind)
		assert.NotEmpty(t, ran.Op, "the refusal still names its operation")
		assert.Empty(t, rec.Taken())
		assert.Empty(t, rec.Published.All())
		assert.Empty(t, reported.All())
	})

	t.Run("failed_apply_finishes_units", func(t *testing.T) {
		rec := &Locks{}
		s := subject(t, service(rec))
		s.Fail(t)
		reported := &Recorder{}

		_, err := s.Run(t.Context(), domain.RunOptions{Surface: "cli", Report: reported.Add})

		require.NoError(t, err, "a failure during the apply is in the outcome, not a refusal")
		inOrder(t, rec, reported.All(), false)
	})
}

// inOrder checks the events of an apply: published as reported, opened by OperationStarted, closed by OperationFinished OK or not, with every unit and phase started finished in between, and the locks released.
func inOrder(t *testing.T, rec *Locks, reported []domain.Event, wantOK bool) {
	t.Helper()
	published := rec.Published.All()
	require.NotEmpty(t, published, "the run published no event")
	assert.IsType(t, domain.OperationStarted{}, published[0])
	finished, ok := published[len(published)-1].(domain.OperationFinished)
	require.True(t, ok, "the last event is %T, want domain.OperationFinished", published[len(published)-1])
	assert.Equal(t, wantOK, finished.OK, "OperationFinished.OK (code %q)", finished.Code)
	assert.Equal(t, published, reported)
	assert.True(t, slices.ContainsFunc(published, func(e domain.Event) bool { _, ok := e.(domain.UnitStarted); return ok }), "no unit started: nothing to check the order of")
	assert.Empty(t, unfinished(published))
	assert.True(t, rec.Released(), "a lock is still held")
}

// unfinished names the units and phases started in events and not finished after.
func unfinished(events []domain.Event) []string {
	var open []string
	for _, event := range events {
		switch e := event.(type) {
		case domain.UnitStarted:
			open = append(open, "unit "+e.Subject)
		case domain.UnitFinished:
			open = finish(open, "unit "+e.Subject)
		case domain.PhaseStarted:
			open = append(open, fmt.Sprintf("phase %s of %s", e.Phase, e.Subject))
		case domain.PhaseFinished:
			open = finish(open, fmt.Sprintf("phase %s of %s", e.Phase, e.Subject))
		case domain.Output, domain.WorktreeCreated, domain.WorktreeUpdated, domain.WorktreeChanged, domain.WorktreeProvisioned, domain.WorktreeRemoved, domain.OperationStarted, domain.OperationFinished, domain.Snapshot, domain.Ready, domain.RepoAdded, domain.RepoRemoved, domain.JobStarted, domain.JobStopped, domain.JobCrashed, domain.JobExited:
		}
	}
	return open
}

// finish removes the first of open named name: a unit started twice is finished twice.
func finish(open []string, name string) []string {
	if i := slices.Index(open, name); i >= 0 {
		return slices.Delete(open, i, i+1)
	}
	return open
}

// service is the operations service over locks.
func service(locks *Locks) operation.Service {
	return operation.Service{
		Locker:    locks,
		Publisher: locks,
		NewOpID:   func() string { return fmt.Sprintf("op-%d", locks.held()+1) },
		Now:       func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) },
	}
}
