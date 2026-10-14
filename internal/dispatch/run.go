package dispatch

import (
	"context"
	"reflect"
	"slices"

	"github.com/LucasPcq/wtm/internal/kernel"
)

type Surface string

const (
	SurfaceCLI  Surface = "cli"
	SurfaceTUI  Surface = "tui"
	SurfaceHTTP Surface = "http"
	SurfaceGUI  Surface = "gui"
)

type Options struct {
	DryRun            bool
	ExpectFingerprint Fingerprint
	IgnoreRemembered  bool
	Surface           Surface
	CorrelationID     string
}

type RunParams[Req any] struct {
	Request  Req
	Options  Options
	Progress kernel.Emitter
}

type Result[P, D any] struct {
	Op          kernel.OpID
	Fingerprint Fingerprint
	Plan        kernel.Plan[P]
	Outcome     *kernel.Outcome[D]
}

func (h *Handle[Req, F, P, D]) Run(ctx context.Context, params RunParams[Req]) (Result[P, D], error) {
	keys := h.locks(params.Request)
	operation := h.operation(operationParams[Req]{Run: params, Keys: keys})
	op := kernel.OpID(operation.ID)
	if params.Options.DryRun {
		result, err := h.run(ctx, runParams[Req]{RunParams: params})
		result.Op = op
		return result, err
	}
	taken, err := h.dispatcher.lock(lockParams{Operation: operation, Keys: keys})
	if err != nil {
		return Result[P, D]{Op: op}, err
	}
	defer taken.release()

	result, err := h.run(ctx, runParams[Req]{RunParams: params, Held: taken})
	result.Op = op
	if taken.protected() {
		return result, err
	}
	unprotected := kernel.Warning{Code: kernel.CodeLocksUnavailable}
	result.Plan.Warnings = append(result.Plan.Warnings, unprotected)
	if result.Outcome != nil {
		result.Outcome.Warnings = append(result.Outcome.Warnings, unprotected)
	}
	return result, err
}

func (h *Handle[Req, F, P, D]) locks(request Req) []kernel.LockKey {
	if h.command.Locks == nil {
		return nil
	}
	return sortedKeys(h.command.Locks(request))
}

type runParams[Req any] struct {
	RunParams[Req]
	Held *held // nil on a dry run
}

type settled[Req, F any] struct {
	Snapshot Snapshot[F]
	Form     kernel.Form[Req]
}

func (h *Handle[Req, F, P, D]) settle(ctx context.Context, params runParams[Req]) (settled[Req, F], error) {
	for {
		snapshot, err := h.Observe(ctx)
		if err != nil {
			return settled[Req, F]{}, err
		}
		form, err := h.State(FormParams[Req, F]{Request: params.Request, Facts: snapshot.Facts, IgnoreRemembered: params.Options.IgnoreRemembered})
		if err != nil {
			return settled[Req, F]{Snapshot: snapshot}, err
		}
		if !form.Complete {
			return settled[Req, F]{Snapshot: snapshot}, kernel.Invalid(form.Errors)
		}
		if params.Held == nil {
			return settled[Req, F]{Snapshot: snapshot, Form: form}, nil
		}
		// A lock the defaults name and the request did not is taken, then the facts
		// observed again: another process may have changed what it guards in between.
		grown, err := params.Held.grow(h.locks(form.Request))
		if err != nil || !grown {
			return settled[Req, F]{Snapshot: snapshot, Form: form}, err
		}
	}
}

func (h *Handle[Req, F, P, D]) run(ctx context.Context, params runParams[Req]) (Result[P, D], error) {
	ready, err := h.settle(ctx, params)
	if err != nil {
		return Result[P, D]{Fingerprint: ready.Snapshot.Fingerprint}, err
	}
	facts := ready.Snapshot.Facts
	if params.Options.IgnoreRemembered {
		facts = h.Forget(facts)
	}
	plan, err := h.command.Plan(ctx, ready.Form.Request, facts)
	if err != nil {
		return Result[P, D]{Fingerprint: ready.Snapshot.Fingerprint}, classify(err)
	}
	result := Result[P, D]{Fingerprint: ready.Snapshot.Fingerprint, Plan: plan}
	if params.Options.DryRun {
		return result, nil
	}
	if params.Options.ExpectFingerprint != "" && params.Options.ExpectFingerprint != ready.Snapshot.Fingerprint {
		return result, kernel.Precondition(kernel.Problem{Code: kernel.CodePlanStale, Params: kernel.Params{
			kernel.ParamExpected: string(params.Options.ExpectFingerprint),
			kernel.ParamFound:    string(ready.Snapshot.Fingerprint),
		}})
	}

	bus := h.dispatcher.bus(ctx, params.Held.operation)
	bus.started()
	progress := newProgress(progressParams{Bus: bus, Forward: params.Progress})
	defer progress.close()
	outcome, err := h.command.Apply(ctx, kernel.ApplyInput[Req, F, P, D]{
		Request:    ready.Form.Request,
		Facts:      facts,
		Plan:       plan,
		Emit:       progress,
		Shield:     h.dispatcher.params.Shield,
		Milestones: milestones(milestonesParams[F, D]{Bus: bus, Facts: facts, Events: h.events}),
	})
	if err != nil {
		failure := classify(err)
		bus.finished(failure)
		return result, failure
	}
	outcome.Warnings = once(append(slices.Clone(plan.Warnings), outcome.Warnings...))
	bus.finished(outcome.Failure())
	result.Outcome = &outcome
	return result, nil
}

func once(warnings []kernel.Warning) []kernel.Warning {
	var kept []kernel.Warning
	for _, warning := range warnings {
		if !slices.ContainsFunc(kept, func(seen kernel.Warning) bool { return reflect.DeepEqual(seen, warning) }) {
			kept = append(kept, warning)
		}
	}
	return kept
}
