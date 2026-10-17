package app

import (
	"context"
	"maps"
	"os"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
)

const commandCreate = "create"

type CreateParams struct {
	Request           create.Request
	DryRun            bool
	ExpectFingerprint string
	Report            wtm.Report
}

type CreateOutcome struct {
	Op          string
	Plan        create.Planned
	Fingerprint string
	Warnings    []wtm.Warning
	Result      *create.Result
}

func (a *App) CreateQuestions(ctx context.Context, req create.Request) (create.Questions, error) {
	facts, err := a.deps.Create.Observe(ctx, req.Branches)
	if err != nil {
		return create.Questions{}, err
	}
	return create.Form(facts, req), nil
}

func (a *App) Create(ctx context.Context, params CreateParams) (CreateOutcome, error) {
	op := a.operation(params.Request.Locks())
	if params.DryRun {
		settled, err := a.settle(ctx, settling{Request: params.Request})
		if err != nil {
			return CreateOutcome{Op: op.ID}, err
		}
		return a.planned(op, settled)
	}
	held, err := a.deps.TakeLocks(op)
	if err != nil {
		return CreateOutcome{Op: op.ID}, err
	}
	defer held.Release()
	settled, err := a.settle(ctx, settling{Request: params.Request, Held: held, Operation: op})
	if err != nil {
		return CreateOutcome{Op: op.ID}, err
	}
	outcome, err := a.planned(op, settled)
	if err != nil {
		return outcome, err
	}
	if params.ExpectFingerprint != "" && params.ExpectFingerprint != outcome.Fingerprint {
		return outcome, &wtm.Error{
			Kind:    wtm.KindPrecondition,
			Code:    "plan.stale",
			Message: "the facts changed since the preview",
			Params:  wtm.Params{"expected": params.ExpectFingerprint, "found": outcome.Fingerprint},
		}
	}
	result := a.apply(ctx, applying{Operation: settled.Operation, Settled: settled, Plan: outcome.Plan, Report: params.Report})
	outcome.Result = &result
	outcome.Warnings = once(append(outcome.Warnings, result.Warnings...))
	if !held.Protected() {
		outcome.Warnings = append(outcome.Warnings, wtm.Warning{Code: "locks.unavailable", Message: "ran without operation locks"})
	}
	return outcome, nil
}

func (a *App) operation(locks []operation.Lock) operation.Operation {
	return operation.Operation{
		ID:            a.deps.NewOpID(),
		PID:           os.Getpid(),
		Command:       commandCreate,
		Surface:       a.deps.Surface,
		CorrelationID: a.deps.CorrelationID,
		StartedAt:     a.deps.Now().UTC(),
		Locks:         locks,
	}
}

type settling struct {
	Request   create.Request
	Held      Hold
	Operation operation.Operation
}

type settled struct {
	Facts     create.Facts
	Request   create.Request
	Operation operation.Operation
}

func (a *App) settle(ctx context.Context, s settling) (settled, error) {
	facts, err := a.deps.Create.Observe(ctx, s.Request.Branches)
	if err != nil {
		return settled{}, err
	}
	resolved, err := create.Resolve(facts, s.Request)
	if err != nil {
		return settled{}, err
	}
	if s.Held == nil {
		return settled{Facts: facts, Request: resolved, Operation: s.Operation}, nil
	}
	grown := s.Operation
	grown.Locks = resolved.Locks()
	taken, err := s.Held.Grow(grown)
	if err != nil {
		return settled{}, err
	}
	if !taken {
		return settled{Facts: facts, Request: resolved, Operation: s.Operation}, nil
	}
	// A lock the defaults named and the request did not is taken first, then the facts are observed again: what it guards may have changed meanwhile.
	return a.settle(ctx, settling{Request: s.Request, Held: s.Held, Operation: grown})
}

func (a *App) planned(op operation.Operation, s settled) (CreateOutcome, error) {
	plan, warnings, err := create.Plan(s.Facts, s.Request)
	if err != nil {
		return CreateOutcome{Op: op.ID}, err
	}
	fingerprint, err := fingerprintOf(plan)
	if err != nil {
		return CreateOutcome{Op: op.ID}, err
	}
	return CreateOutcome{Op: op.ID, Plan: plan, Fingerprint: fingerprint, Warnings: warnings}, nil
}

type applying struct {
	Operation operation.Operation
	Settled   settled
	Plan      create.Planned
	Report    wtm.Report
}

func (a *App) apply(ctx context.Context, run applying) create.Result {
	publisher := a.deps.Publisher(run.Operation)
	report := func(event wtm.Event) {
		publisher.Publish(event)
		run.Report(event)
	}
	report(wtm.OperationStarted{Command: commandCreate, Surface: a.deps.Surface})
	result := create.Apply(ctx, create.ApplyParams{
		Deps:        a.deps.Create,
		Facts:       run.Settled.Facts,
		Plan:        run.Plan,
		Isolation:   run.Settled.Request.Isolation,
		FastForward: *run.Settled.Request.FastForward,
		Report:      report,
	})
	finished := wtm.OperationFinished{Command: commandCreate, Surface: a.deps.Surface, OK: true}
	if failure := result.Failure(); failure != nil {
		finished.OK, finished.Code = false, failure.Code
	}
	report(finished)
	return result
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
