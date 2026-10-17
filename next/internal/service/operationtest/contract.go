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
		assert.Empty(t, rec.taken)
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
		published := rec.published.list
		require.NotEmpty(t, published, "the run published no event")
		assert.IsType(t, domain.OperationStarted{}, published[0])
		assert.IsType(t, domain.OperationFinished{}, published[len(published)-1])
		assert.Equal(t, published, reported.list)
		assert.True(t, slices.ContainsFunc(published, func(e domain.Event) bool { _, ok := e.(domain.UnitStarted); return ok }), "no unit started: nothing to check the order of")
		assert.Empty(t, unfinished(published))
		assert.True(t, rec.released(), "a lock is still held")
	})
}

// unfinished names the units and phases started in events and not finished after.
func unfinished(events []domain.Event) []string {
	var open []string
	for _, event := range events {
		switch e := event.(type) {
		case domain.UnitStarted:
			open = append(open, "unit "+e.Subject)
		case domain.UnitFinished:
			open = slices.DeleteFunc(open, func(name string) bool { return name == "unit "+e.Subject })
		case domain.PhaseStarted:
			open = append(open, fmt.Sprintf("phase %s of %s", e.Phase, e.Subject))
		case domain.PhaseFinished:
			open = slices.DeleteFunc(open, func(name string) bool { return name == fmt.Sprintf("phase %s of %s", e.Phase, e.Subject) })
		case domain.Output, domain.WorktreeCreated, domain.WorktreeNumbered, domain.WorktreeProvisioned, domain.OperationStarted, domain.OperationFinished:
		}
	}
	return open
}

// ----------------------------------------------------------------------------
// Fakes

// recorder is the Locker and the Publisher of an operations service, recording the operations that took locks, the holds they got and what they published.
type recorder struct {
	taken     []domain.Operation
	holds     []*hold
	published events
}

func (r *recorder) service() operation.Service {
	return operation.Service{
		Locker:    r,
		Publisher: r,
		NewOpID:   func() string { return fmt.Sprintf("op-%d", len(r.taken)+1) },
		Now:       func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) },
	}
}

func (r *recorder) Take(op domain.Operation) (operation.Hold, error) {
	r.taken = append(r.taken, op)
	h := &hold{held: slices.Clone(op.Locks)}
	r.holds = append(r.holds, h)
	return h, nil
}

func (r *recorder) Open(domain.Operation) domain.Report {
	return r.published.add
}

func (r *recorder) released() bool {
	return !slices.ContainsFunc(r.holds, func(h *hold) bool { return !h.released })
}

type hold struct {
	held     []domain.Lock
	released bool
}

func (h *hold) Grow(locks []domain.Lock) (bool, error) {
	grown := false
	for _, lock := range locks {
		if !slices.Contains(h.held, lock) {
			h.held = append(h.held, lock)
			grown = true
		}
	}
	return grown, nil
}

func (h *hold) Protected() bool { return true }

func (h *hold) Release() { h.released = true }

type events struct{ list []domain.Event }

func (e *events) add(event domain.Event) { e.list = append(e.list, event) }
