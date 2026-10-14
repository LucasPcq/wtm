package dispatch

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain/operation"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
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

// Result carries no Outcome after a dry run.
type Result[P, D any] struct {
	Op          kernel.OpID
	Fingerprint Fingerprint
	Plan        kernel.Plan[P]
	Outcome     *kernel.Outcome[D]
}

// Run holds the locks from before Observe to the end of Apply, so no other
// wtm process changes the facts the plan was computed on. A dry run takes
// none: the fingerprint guards the run that applies its plan.
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

type operationParams[Req any] struct {
	Run  RunParams[Req]
	Keys []kernel.LockKey
}

func (h *Handle[Req, F, P, D]) operation(params operationParams[Req]) operation.Operation {
	return operation.Operation{
		ID:            string(h.dispatcher.params.NewOpID()),
		PID:           os.Getpid(),
		Command:       h.command.Name,
		Surface:       string(params.Run.Options.Surface),
		CorrelationID: params.Run.Options.CorrelationID,
		StartedAt:     h.dispatcher.params.Now().UTC(),
		Locks:         recorded(params.Keys),
	}
}

func recorded(keys []kernel.LockKey) []operation.Lock {
	if keys == nil {
		return nil
	}
	locks := make([]operation.Lock, 0, len(keys))
	for _, key := range keys {
		locks = append(locks, operation.Lock{Scope: string(key.Scope), Name: key.Name, Mode: string(key.Mode)})
	}
	return locks
}

func (h *Handle[Req, F, P, D]) locks(request Req) []kernel.LockKey {
	if h.command.Locks == nil {
		return nil
	}
	return sortedKeys(h.command.Locks(request))
}

// runParams: Held is nil on a dry run.
type runParams[Req any] struct {
	RunParams[Req]
	Held *held
}

// settled is a complete request on the facts observed under every lock it
// needs.
type settled[Req, F any] struct {
	Snapshot Snapshot[F]
	Form     kernel.Form[Req]
}

// settle observes and fills in the defaults; when they name locks the request
// as received did not, it takes them and observes again, since another
// process may have changed what they guard in between.
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

// once keeps the first of each warning given twice: a command may repeat in
// its outcome a warning of its plan.
func once(warnings []kernel.Warning) []kernel.Warning {
	var kept []kernel.Warning
	for _, warning := range warnings {
		if !slices.ContainsFunc(kept, func(seen kernel.Warning) bool { return reflect.DeepEqual(seen, warning) }) {
			kept = append(kept, warning)
		}
	}
	return kept
}

func classify(err error) kernel.Error {
	var newer *statefile.NewerError
	if errors.As(err, &newer) {
		return kernel.Precondition(kernel.Problem{Code: kernel.CodeStateSchemaNewer, Cause: err, Params: kernel.Params{
			kernel.ParamPath:      newer.Path,
			kernel.ParamFound:     strconv.Itoa(newer.Found),
			kernel.ParamSupported: strconv.Itoa(newer.Supported),
		}})
	}
	return kernel.Classify(kernel.ClassifyParams{Err: err, Code: kernel.CodeInternal})
}
