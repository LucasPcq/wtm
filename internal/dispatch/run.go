package dispatch

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

type RunParams[Req any] struct {
	Request Req
	Options Options
	// Progress receives what the command emits while it applies; nil drops it.
	Progress kernel.Emitter
}

// Result is a run's answer: the plan and the fingerprint of the facts it was
// computed on always, the outcome unless the run was a dry run.
type Result[P, D any] struct {
	Op          kernel.OpID
	Fingerprint Fingerprint
	Plan        kernel.Plan[P]
	Outcome     *kernel.Outcome[D]
}

// Run takes the command's locks, then observes, fills in the defaults,
// validates, plans and, unless it is a dry run or the facts changed since the
// preview, applies, before letting the locks go. Every command runs through
// these same steps, whatever the surface.
func (h *Handle[Req, F, P, D]) Run(ctx context.Context, params RunParams[Req]) (Result[P, D], error) {
	operation := h.operation(params)
	taken, err := h.dispatcher.lock(operation)
	if err != nil {
		return Result[P, D]{Op: operation.ID}, err
	}
	defer taken.release()

	result, err := h.run(ctx, runParams[Req]{RunParams: params, Operation: operation})
	result.Op = operation.ID
	if taken.protected() {
		return result, err
	}
	unprotected := kernel.Warning{Code: CodeLocksUnavailable}
	result.Plan.Warnings = append(result.Plan.Warnings, unprotected)
	if result.Outcome != nil {
		result.Outcome.Warnings = append(result.Outcome.Warnings, unprotected)
	}
	return result, err
}

func (h *Handle[Req, F, P, D]) operation(params RunParams[Req]) kernel.Operation {
	var keys []kernel.LockKey
	if h.command.Locks != nil {
		keys = sortedKeys(h.command.Locks(params.Request))
	}
	return kernel.Operation{
		ID:            h.dispatcher.params.NewOpID(),
		PID:           os.Getpid(),
		Command:       h.command.Name,
		Surface:       params.Options.Surface,
		CorrelationID: params.Options.CorrelationID,
		StartedAt:     h.dispatcher.params.Now().UTC(),
		Locks:         keys,
	}
}

type runParams[Req any] struct {
	RunParams[Req]
	Operation kernel.Operation
}

func (h *Handle[Req, F, P, D]) run(ctx context.Context, params runParams[Req]) (Result[P, D], error) {
	snapshot, err := h.Observe(ctx)
	if err != nil {
		return Result[P, D]{}, err
	}
	facts := snapshot.Facts
	if params.Options.IgnoreRemembered {
		facts = h.Forget(facts)
	}
	form, err := h.State(FormParams[Req, F]{Request: params.Request, Facts: facts})
	if err != nil {
		return Result[P, D]{Fingerprint: snapshot.Fingerprint}, err
	}
	if !form.Complete {
		return Result[P, D]{Fingerprint: snapshot.Fingerprint}, kernel.Invalid(form.Errors)
	}
	plan, err := h.command.Plan(ctx, form.Request, facts)
	if err != nil {
		return Result[P, D]{Fingerprint: snapshot.Fingerprint}, classify(err)
	}
	result := Result[P, D]{Fingerprint: snapshot.Fingerprint, Plan: plan}
	if params.Options.DryRun {
		return result, nil
	}
	if params.Options.ExpectFingerprint != "" && params.Options.ExpectFingerprint != snapshot.Fingerprint {
		return result, kernel.Precondition(kernel.Problem{Code: CodePlanStale, Params: kernel.Params{
			ParamExpected: string(params.Options.ExpectFingerprint),
			ParamFound:    string(snapshot.Fingerprint),
		}})
	}

	bus := h.dispatcher.bus(ctx, params.Operation)
	bus.started()
	progress := h.dispatcher.progress(progressParams{Bus: bus, Forward: params.Progress})
	defer progress.close()
	outcome, err := h.command.Apply(ctx, kernel.ApplyInput[Req, F, P]{
		Request: form.Request,
		Facts:   facts,
		Plan:    plan,
		Emit:    progress,
		Shield:  h.dispatcher.params.Shield,
	})
	if err != nil {
		failure := classify(err)
		bus.finished(failure)
		return result, failure
	}
	bus.finished(nil)
	result.Outcome = &outcome
	return result, nil
}

// classify files any error under the taxonomy; a state file written by a
// newer wtm is a precondition, carrying the file and both schemas.
func classify(err error) kernel.Error {
	var newer *statefile.NewerError
	if errors.As(err, &newer) {
		return kernel.Precondition(kernel.Problem{Code: CodeStateSchemaNewer, Cause: err, Params: kernel.Params{
			kernel.ParamPath: newer.Path,
			ParamFound:       strconv.Itoa(newer.Found),
			ParamSupported:   strconv.Itoa(newer.Supported),
		}})
	}
	return kernel.Classify(kernel.ClassifyParams{Err: err, Code: kernel.CodeInternal})
}
