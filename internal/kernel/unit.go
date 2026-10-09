package kernel

import (
	"context"
	"errors"
	"strings"
)

// Shield returns a context no cancellation reaches and its release; dispatch
// hands in infra's, which also keeps the process alive until it is released.
type Shield func(context.Context) (context.Context, func())

// Prep runs before the saga and may be interrupted: nothing has changed yet.
type Prep struct {
	Name string
	Run  func(context.Context) error
}

type SagaStep struct {
	Name string
	Do   func(context.Context) error
	Undo func(context.Context) error
}

// Saga is all or nothing: it runs shielded to the end, and a failure undoes
// the steps done, in reverse. Commit runs last, so nothing announces what
// could still be undone.
type Saga[D any] struct {
	Steps      []SagaStep
	Commit     func(context.Context) (D, error)
	LeftBehind func(step string) *FollowUp
}

// Phase runs after the commit: interruptible, never undone.
type Phase[D any] struct {
	Name string
	Run  func(context.Context, D) (D, error)
}

type Unit[D any] struct {
	Before []Prep
	Saga   Saga[D]
	Then   []Phase[D]
}

type EachParams[T, D any] struct {
	Items         []T
	Subject       func(T) string
	Unit          func(T) Unit[D]
	Emit          Emitter
	Shield        Shield
	StopOnFailure bool
}

// Each runs one unit per item. ctx is checked before each unit: the units
// never reached are skipped, interrupted.
func Each[T, D any](ctx context.Context, params EachParams[T, D]) []Item[D] {
	items := make([]Item[D], 0, len(params.Items))
	for index, it := range params.Items {
		if ctx.Err() != nil {
			return append(items, unreached(unreachedParams[T, D]{Each: params, From: index, Reason: ReasonInterrupted})...)
		}
		run := unitRun[D]{subject: params.Subject(it), emit: emitterOr(params.Emit), shield: shieldOr(params.Shield)}
		item := run.unit(ctx, params.Unit(it))
		items = append(items, item)
		if item.Status == StatusFailed && params.StopOnFailure {
			return append(items, unreached(unreachedParams[T, D]{Each: params, From: index + 1, Reason: ReasonNotReached})...)
		}
	}
	return items
}

type unreachedParams[T, D any] struct {
	Each   EachParams[T, D]
	From   int
	Reason Reason
}

func unreached[T, D any](params unreachedParams[T, D]) []Item[D] {
	rest := params.Each.Items[params.From:]
	items := make([]Item[D], 0, len(rest))
	for _, it := range rest {
		items = append(items, Item[D]{Subject: params.Each.Subject(it), Status: StatusSkipped, Reason: params.Reason})
	}
	return items
}

type unitRun[D any] struct {
	subject string
	emit    Emitter
	shield  Shield
}

func (r unitRun[D]) unit(ctx context.Context, unit Unit[D]) Item[D] {
	r.emit.Emit(Progress{Subject: r.subject, Kind: ProgressUnitStarted})
	item := r.body(ctx, unit)
	r.emit.Emit(Progress{Subject: r.subject, Kind: ProgressUnitFinished, Params: map[string]string{ParamStatus: string(item.Status)}})
	return item
}

func (r unitRun[D]) body(ctx context.Context, unit Unit[D]) Item[D] {
	var none D
	for _, prep := range unit.Before {
		if err := r.phase(ctx, phaseRun{name: prep.Name, run: prep.Run}); err != nil {
			return r.ended(ended[D]{ctx: ctx, err: err, phase: prep.Name, code: CodePhaseFailed, detail: none})
		}
	}
	if ctx.Err() != nil {
		return r.ended(ended[D]{ctx: ctx, err: ctx.Err(), detail: none})
	}
	detail, failure := r.saga(ctx, unit.Saga)
	if failure != nil {
		return Item[D]{Subject: r.subject, Status: StatusFailed, Error: failure, Detail: none}
	}
	for _, then := range unit.Then {
		next, err := r.then(ctx, phaseOf[D]{phase: then, detail: detail})
		if err != nil {
			return r.ended(ended[D]{ctx: ctx, err: err, phase: then.Name, code: CodePhaseFailed, detail: detail})
		}
		detail = next
	}
	return Item[D]{Subject: r.subject, Status: StatusDone, Detail: detail}
}

type phaseRun struct {
	name string
	run  func(context.Context) error
}

func (r unitRun[D]) phase(ctx context.Context, phase phaseRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	finished := r.announce(phase.name)
	defer finished()
	return phase.run(ctx)
}

type phaseOf[D any] struct {
	phase  Phase[D]
	detail D
}

func (r unitRun[D]) then(ctx context.Context, step phaseOf[D]) (D, error) {
	if err := ctx.Err(); err != nil {
		return step.detail, err
	}
	finished := r.announce(step.phase.Name)
	defer finished()
	next, err := step.phase.Run(ctx, step.detail)
	if err != nil {
		return step.detail, err
	}
	return next, nil
}

func (r unitRun[D]) announce(name string) func() {
	params := map[string]string{ParamPhase: name}
	r.emit.Emit(Progress{Subject: r.subject, Kind: ProgressPhaseStarted, Params: params})
	return func() { r.emit.Emit(Progress{Subject: r.subject, Kind: ProgressPhaseFinished, Params: params}) }
}

func (r unitRun[D]) saga(ctx context.Context, saga Saga[D]) (D, *Error) {
	var none D
	shielded, release := r.shield(ctx)
	defer release()
	for index, step := range saga.Steps {
		if err := r.phase(shielded, phaseRun{name: step.Name, run: step.Do}); err != nil {
			failure := Classify(ClassifyParams{Err: err, Code: CodeStepFailed, Params: map[string]string{ParamStep: step.Name}})
			return none, undo(shielded, undoParams[D]{Saga: saga, Done: index, Failure: failure})
		}
	}
	if saga.Commit == nil {
		return none, nil
	}
	detail, err := saga.Commit(shielded)
	if err != nil {
		failure := Classify(ClassifyParams{Err: err, Code: CodeStepFailed, Params: map[string]string{ParamStep: "commit"}})
		return none, undo(shielded, undoParams[D]{Saga: saga, Done: len(saga.Steps), Failure: failure})
	}
	return detail, nil
}

type undoParams[D any] struct {
	Saga    Saga[D]
	Done    int
	Failure *Error
}

// undo walks back the steps done; the first Undo that fails stops it, and the
// error names that step and everything still standing.
func undo[D any](ctx context.Context, params undoParams[D]) *Error {
	for index := params.Done - 1; index >= 0; index-- {
		step := params.Saga.Steps[index]
		if step.Undo == nil {
			continue
		}
		if err := step.Undo(ctx); err != nil {
			return stuck(stuckParams[D]{Saga: params.Saga, At: index, Failure: params.Failure, Err: err})
		}
	}
	return params.Failure
}

type stuckParams[D any] struct {
	Saga    Saga[D]
	At      int
	Failure *Error
	Err     error
}

func stuck[D any](params stuckParams[D]) *Error {
	names := make([]string, 0, params.At+1)
	for _, step := range params.Saga.Steps[:params.At+1] {
		names = append(names, step.Name)
	}
	name := params.Saga.Steps[params.At].Name
	stuck := &Error{
		Kind:   KindInternal,
		Code:   CodeUndoFailed,
		Params: map[string]string{ParamStep: name, ParamLeft: strings.Join(names, ",")},
		Cause:  errors.Join(params.Failure, params.Err),
	}
	if params.Saga.LeftBehind != nil {
		stuck.FollowUp = params.Saga.LeftBehind(name)
	}
	return stuck
}

type ended[D any] struct {
	ctx    context.Context
	err    error
	phase  string
	code   Code
	detail D
}

// ended files a unit stopped outside its saga: a failure the interruption
// caused counts as cancelled, whatever error it surfaced as.
func (r unitRun[D]) ended(end ended[D]) Item[D] {
	params := phaseParam(end.phase)
	if end.ctx.Err() != nil {
		cancelled := &Error{Kind: KindCancelled, Code: CodeInterrupted, Params: params, Cause: end.err}
		return Item[D]{Subject: r.subject, Status: StatusCancelled, Reason: ReasonInterrupted, Error: cancelled, Detail: end.detail}
	}
	failure := Classify(ClassifyParams{Err: end.err, Code: end.code, Params: params})
	return Item[D]{Subject: r.subject, Status: StatusFailed, Error: failure, Detail: end.detail}
}

func phaseParam(phase string) map[string]string {
	if phase == "" {
		return nil
	}
	return map[string]string{ParamPhase: phase}
}

func emitterOr(emit Emitter) Emitter {
	if emit == nil {
		return discard{}
	}
	return emit
}

func shieldOr(shield Shield) Shield {
	if shield != nil {
		return shield
	}
	return func(ctx context.Context) (context.Context, func()) {
		return context.WithoutCancel(ctx), func() {}
	}
}
