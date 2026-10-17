package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const (
	CodePlanStale    = "plan.stale"
	fingerprintBytes = 12
)

type plan interface {
	Locks() []wtm.Lock
	Fingerprinted() any
}

type result interface {
	Failure() *wtm.Error
	Warned() []wtm.Warning
}

type steps[P plan, R result] struct {
	Command string
	Locks   []wtm.Lock
	Plan    func(ctx context.Context) (P, []wtm.Warning, error)
	Apply   func(ctx context.Context, plan P, report wtm.Report) R
}

type Params struct {
	DryRun            bool
	ExpectFingerprint string
	Surface           string
	Report            wtm.Report
}

type Outcome[P plan, R result] struct {
	Op          string
	Plan        P
	Fingerprint string
	Warnings    []wtm.Warning
	Result      *R
}

func (o Outcome[P, R]) Failure() *wtm.Error {
	if o.Result == nil {
		return nil
	}
	return (*o.Result).Failure()
}

func run[P plan, R result](ctx context.Context, a *App, s steps[P, R], params Params) (Outcome[P, R], error) {
	op := a.operation(operating{Command: s.Command, Locks: s.Locks, Surface: params.Surface})
	if params.DryRun {
		planned, warnings, err := s.Plan(ctx)
		if err != nil {
			return Outcome[P, R]{Op: op.ID}, err
		}
		return fingerprinted[P, R](op, planned, warnings)
	}
	held, err := a.deps.TakeLocks(op)
	if err != nil {
		return Outcome[P, R]{Op: op.ID}, err
	}
	defer held.Release()
	planned, warnings, err := underLocks(ctx, s.Plan, held)
	if err != nil {
		return Outcome[P, R]{Op: op.ID}, err
	}
	outcome, err := fingerprinted[P, R](op, planned, warnings)
	if err != nil {
		return outcome, err
	}
	if params.ExpectFingerprint != "" && params.ExpectFingerprint != outcome.Fingerprint {
		return outcome, &wtm.Error{
			Kind:    wtm.KindPrecondition,
			Code:    CodePlanStale,
			Message: "the facts changed since the preview",
			Params:  wtm.Params{"expected": params.ExpectFingerprint, "found": outcome.Fingerprint},
		}
	}
	report := a.report(op, params.Report)
	report(wtm.OperationStarted{Command: s.Command, Surface: op.Surface})
	applied := s.Apply(ctx, outcome.Plan, report)
	finished := wtm.OperationFinished{Command: s.Command, Surface: op.Surface, OK: true}
	if failure := applied.Failure(); failure != nil {
		finished.OK, finished.Code = false, failure.Code
	}
	report(finished)
	outcome.Result = &applied
	outcome.Warnings = append(outcome.Warnings, applied.Warned()...)
	if !held.Protected() {
		outcome.Warnings = append(outcome.Warnings, wtm.LocksUnavailable)
	}
	outcome.Warnings = once(outcome.Warnings)
	return outcome, nil
}

type operating struct {
	Command string
	Locks   []wtm.Lock
	Surface string
}

func (a *App) operation(o operating) wtm.Operation {
	return wtm.Operation{
		ID:            a.deps.NewOpID(),
		PID:           os.Getpid(),
		Command:       o.Command,
		Surface:       o.Surface,
		CorrelationID: a.deps.CorrelationID,
		StartedAt:     a.deps.Now().UTC(),
		Locks:         o.Locks,
	}
}

func underLocks[P plan](ctx context.Context, planOf func(context.Context) (P, []wtm.Warning, error), held Hold) (P, []wtm.Warning, error) {
	for {
		planned, warnings, err := planOf(ctx)
		if err != nil {
			return planned, nil, err
		}
		grown, err := held.Grow(planned.Locks())
		if err != nil || !grown {
			return planned, warnings, err
		}
		// A lock the defaults named and the request did not is taken first, then the facts are observed again: what it guards may have changed meanwhile.
	}
}

func fingerprinted[P plan, R result](op wtm.Operation, planned P, warnings []wtm.Warning) (Outcome[P, R], error) {
	data, err := json.Marshal(planned.Fingerprinted())
	if err != nil {
		return Outcome[P, R]{Op: op.ID}, err
	}
	sum := sha256.Sum256(data)
	return Outcome[P, R]{Op: op.ID, Plan: planned, Fingerprint: hex.EncodeToString(sum[:fingerprintBytes]), Warnings: warnings}, nil
}

func (a *App) report(op wtm.Operation, surface wtm.Report) wtm.Report {
	publish := a.deps.Publisher(op)
	return func(event wtm.Event) {
		publish(event)
		if surface != nil {
			surface(event)
		}
	}
}

func once(warnings []wtm.Warning) []wtm.Warning {
	var kept []wtm.Warning
	for _, warning := range warnings {
		seen := slices.ContainsFunc(kept, func(k wtm.Warning) bool {
			return k.Code == warning.Code && k.Message == warning.Message && maps.Equal(k.Params, warning.Params)
		})
		if !seen {
			kept = append(kept, warning)
		}
	}
	return kept
}
