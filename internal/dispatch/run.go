package dispatch

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

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
// wtm process changes the facts the plan was computed on.
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
	unprotected := kernel.Warning{Code: kernel.CodeLocksUnavailable}
	result.Plan.Warnings = append(result.Plan.Warnings, unprotected)
	if result.Outcome != nil {
		result.Outcome.Warnings = append(result.Outcome.Warnings, unprotected)
	}
	return result, err
}

func (h *Handle[Req, F, P, D]) operation(params RunParams[Req]) operations.Operation {
	var keys []kernel.LockKey
	if h.command.Locks != nil {
		keys = sortedKeys(h.command.Locks(params.Request))
	}
	return operations.Operation{
		ID:            h.dispatcher.params.NewOpID(),
		PID:           os.Getpid(),
		Command:       h.command.Name,
		Surface:       string(params.Options.Surface),
		CorrelationID: params.Options.CorrelationID,
		StartedAt:     h.dispatcher.params.Now().UTC(),
		Locks:         keys,
	}
}

type runParams[Req any] struct {
	RunParams[Req]
	Operation operations.Operation
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
		return result, kernel.Precondition(kernel.Problem{Code: kernel.CodePlanStale, Params: kernel.Params{
			kernel.ParamExpected: string(params.Options.ExpectFingerprint),
			kernel.ParamFound:    string(snapshot.Fingerprint),
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
