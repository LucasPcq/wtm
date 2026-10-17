// Package operationtest is the contract every mutating use case's Run keeps with its operation: a dry run takes no lock and publishes nothing, a stale plan is refused before any event, and the events of an apply arrive in order, each unit started finished.
package operationtest

import (
	"context"
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
	// Run runs a request that applies a change with events, returning the fingerprint of its plan.
	Run func(ctx context.Context, opts domain.RunOptions) (fingerprint string, err error)
	// Stale changes the facts the request's plan depends on, so that a preview made before no longer holds.
	Stale func(t *testing.T)
	// Fail makes the apply of the request fail once it started, as a failing hook does.
	Fail func(t *testing.T)
}

// Contract checks the contract on the Subject that subject returns, a new one for each check, wired to the operations service it is given.
func Contract(t *testing.T, subject func(t *testing.T, ops operation.Service) Subject) {
	t.Run("a dry run takes no lock and publishes nothing", func(t *testing.T) {
		rec := &recorder{}
		run := subject(t, rec.service()).Run
		reported := &events{}

		fingerprint, err := run(t.Context(), domain.RunOptions{DryRun: true, Report: reported.add})

		require.NoError(t, err)
		assert.NotEmpty(t, fingerprint)
		assert.Empty(t, rec.holds)
		assert.Empty(t, rec.published.list)
		assert.Empty(t, reported.list)
	})

	t.Run("a stale plan is refused before any event", func(t *testing.T) {
		rec := &recorder{}
		s := subject(t, rec.service())
		preview, err := s.Run(t.Context(), domain.RunOptions{DryRun: true})
		require.NoError(t, err)
		s.Stale(t)
		reported := &events{}

		_, err = s.Run(t.Context(), domain.RunOptions{ExpectFingerprint: preview, Report: reported.add})

		assert.True(t, domain.PlanStale(err), "Run with a stale fingerprint = %v, want %s", err, domain.CodePlanStale)
		assert.Empty(t, rec.published.list)
		assert.Empty(t, reported.list)
		assert.True(t, rec.released(), "a lock is still held")
	})

	t.Run("events arrive in order and every unit started finishes", func(t *testing.T) {
		rec := &recorder{}
		run := subject(t, rec.service()).Run
		reported := &events{}

		_, err := run(t.Context(), domain.RunOptions{Surface: "cli", Report: reported.add})

		require.NoError(t, err)
		inOrder(t, inOrderParams{Recorder: rec, Reported: reported.list, OK: true})
	})

	t.Run("a failed apply still finishes every unit it started", func(t *testing.T) {
		rec := &recorder{}
		s := subject(t, rec.service())
		s.Fail(t)
		reported := &events{}

		_, err := s.Run(t.Context(), domain.RunOptions{Surface: "cli", Report: reported.add})

		require.NoError(t, err, "a failure during the apply is in the outcome, not a refusal")
		inOrder(t, inOrderParams{Recorder: rec, Reported: reported.list, OK: false})
	})
}

type inOrderParams struct {
	Recorder *recorder
	Reported []domain.Event
	OK       bool // whether the operation finished OK
}

// inOrder checks the events of an apply: published as reported, opened by OperationStarted, closed by OperationFinished, with every unit and phase started finished in between, and the locks released.
func inOrder(t *testing.T, p inOrderParams) {
	t.Helper()
	published := p.Recorder.published.list
	require.NotEmpty(t, published, "the run published no event")
	assert.IsType(t, domain.OperationStarted{}, published[0])
	finished, ok := published[len(published)-1].(domain.OperationFinished)
	require.True(t, ok, "the last event is %T, want domain.OperationFinished", published[len(published)-1])
	assert.Equal(t, p.OK, finished.OK, "OperationFinished.OK (code %q)", finished.Code)
	assert.Equal(t, published, p.Reported)
	assert.True(t, slices.ContainsFunc(published, func(e domain.Event) bool { _, ok := e.(domain.UnitStarted); return ok }), "no unit started: nothing to check the order of")
	assert.Empty(t, unfinished(published))
	assert.True(t, p.Recorder.released(), "a lock is still held")
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
		case domain.Output, domain.WorktreeCreated, domain.WorktreeNumbered, domain.WorktreeProvisioned, domain.OperationStarted, domain.OperationFinished:
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

// ----------------------------------------------------------------------------
// Fakes

// recorder is the Locker and the Publisher of an operations service, recording the holds it gave and what was published.
type recorder struct {
	holds     []*hold
	published events
}

func (r *recorder) service() operation.Service {
	return operation.Service{
		Locker:    r,
		Publisher: r,
		NewOpID:   func() string { return fmt.Sprintf("op-%d", len(r.holds)+1) },
		Now:       func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) },
	}
}

func (r *recorder) Take(op domain.Operation) (operation.Hold, error) {
	h := &hold{}
	r.holds = append(r.holds, h)
	return h, nil
}

func (r *recorder) Open(domain.Operation) domain.Report {
	return r.published.add
}

func (r *recorder) released() bool {
	return !slices.ContainsFunc(r.holds, func(h *hold) bool { return !h.released })
}

// hold never grows: the contract does not check planning again under new locks, PlanUnderLocks's own tests do.
type hold struct{ released bool }

func (h *hold) Grow([]domain.Lock) (bool, error) { return false, nil }

func (h *hold) Protected() bool { return true }

func (h *hold) Release() { h.released = true }

type events struct{ list []domain.Event }

func (e *events) add(event domain.Event) { e.list = append(e.list, event) }
