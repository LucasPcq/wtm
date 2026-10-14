package dispatch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const factsMismatchFmt = "facts of %T handed to %s"

// Fillable is a command's entry as a form fills it: the calls of its Handle
// with the request as JSON and the facts kept opaque, so one form serves every
// command of the registry without knowing its types.
type Fillable interface {
	Entry
	Observe(ctx context.Context) (Observed, error)
	State(params FillParams) (kernel.Form[json.RawMessage], error)
	Choices(ctx context.Context, params FillChoicesParams) (kernel.Choices, error)
	Set(params SetParams) (json.RawMessage, error)
}

// Observed is a snapshot whose facts a surface never reads: it hands them back
// to the entry that observed them.
type Observed struct {
	facts       any
	Fingerprint Fingerprint
}

type FillParams struct {
	Request          json.RawMessage
	Facts            Observed
	IgnoreRemembered bool
}

type FillChoicesParams struct {
	Path             string
	Request          json.RawMessage
	Facts            Observed
	IgnoreRemembered bool
}

type SetParams struct {
	Request json.RawMessage
	Path    string
	Value   kernel.Value
}

// fillable embeds the Handle for Name, Schema and Call; its own methods shadow
// the typed calls of the same name.
type fillable[Req, F, P, D any] struct {
	*Handle[Req, F, P, D]
}

func (h *Handle[Req, F, P, D]) Fillable() (Fillable, bool) {
	return fillable[Req, F, P, D]{h}, true
}

func (*QueryHandle[Req, V]) Fillable() (Fillable, bool) { return nil, false }

func (f fillable[Req, F, P, D]) Observe(ctx context.Context) (Observed, error) {
	snapshot, err := f.Handle.Observe(ctx)
	if err != nil {
		return Observed{}, err
	}
	return Observed{facts: snapshot.Facts, Fingerprint: snapshot.Fingerprint}, nil
}

func (f fillable[Req, F, P, D]) State(params FillParams) (kernel.Form[json.RawMessage], error) {
	request, facts, err := f.inputs(params.Request, params.Facts)
	if err != nil {
		return kernel.Form[json.RawMessage]{}, err
	}
	form, err := f.Handle.State(FormParams[Req, F]{Request: request, Facts: facts, IgnoreRemembered: params.IgnoreRemembered})
	if err != nil {
		return kernel.Form[json.RawMessage]{}, err
	}
	encoded, err := json.Marshal(form.Request)
	if err != nil {
		return kernel.Form[json.RawMessage]{}, classify(err)
	}
	return kernel.Form[json.RawMessage]{Request: encoded, States: form.States, Errors: form.Errors, Complete: form.Complete}, nil
}

func (f fillable[Req, F, P, D]) Choices(ctx context.Context, params FillChoicesParams) (kernel.Choices, error) {
	request, facts, err := f.inputs(params.Request, params.Facts)
	if err != nil {
		return kernel.Choices{}, err
	}
	if params.IgnoreRemembered {
		facts = f.Handle.Forget(facts)
	}
	return f.Handle.Choices(ctx, ChoicesParams[Req, F]{Path: params.Path, Request: request, Facts: facts})
}

// Set goes through the typed request, so a value of the wrong shape for its
// field is refused as kernel.Set refuses it.
func (f fillable[Req, F, P, D]) Set(params SetParams) (json.RawMessage, error) {
	request, err := decodeRequest[Req](params.Request)
	if err != nil {
		return nil, err
	}
	updated, err := kernel.Set(request, params.Path, params.Value)
	if err != nil {
		invalid := kernel.Invalid([]kernel.FieldError{{Path: params.Path, Code: kernel.CodeInvalid}})
		invalid.Cause = err
		return nil, invalid
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		return nil, classify(err)
	}
	return encoded, nil
}

func (f fillable[Req, F, P, D]) inputs(raw json.RawMessage, observed Observed) (Req, F, error) {
	request, err := decodeRequest[Req](raw)
	if err != nil {
		var none F
		return request, none, err
	}
	facts, err := f.facts(observed)
	return request, facts, err
}

func (f fillable[Req, F, P, D]) facts(observed Observed) (F, error) {
	facts, ok := observed.facts.(F)
	if !ok {
		return facts, classify(fmt.Errorf(factsMismatchFmt, observed.facts, f.Name()))
	}
	return facts, nil
}
