package dispatch

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const candidateShapeFmt = "a %s field holds a %T"

type Snapshot[F any] struct {
	Facts       F
	Fingerprint Fingerprint
}

func (h *Handle[Req, F, P, D]) Observe(ctx context.Context) (Snapshot[F], error) {
	var facts F
	if h.command.Observe != nil {
		observed, err := h.command.Observe(ctx)
		if err != nil {
			return Snapshot[F]{}, classify(err)
		}
		facts = observed
	}
	fingerprint, err := fingerprintOf(facts)
	if err != nil {
		return Snapshot[F]{}, classify(err)
	}
	return Snapshot[F]{Facts: facts, Fingerprint: fingerprint}, nil
}

func (h *Handle[Req, F, P, D]) Forget(facts F) F {
	if h.command.Forget == nil {
		return facts
	}
	return h.command.Forget(facts)
}

// FormParams: with IgnoreRemembered, the form is evaluated on the facts the
// command forgets, each field still carrying what they remembered.
type FormParams[Req, F any] struct {
	Request          Req
	Facts            F
	IgnoreRemembered bool
}

func (h *Handle[Req, F, P, D]) State(params FormParams[Req, F]) (kernel.Form[Req], error) {
	evaluate := kernel.EvaluateParams[Req, F]{
		Fields:  h.command.Fields,
		Rules:   h.command.Rules,
		Request: params.Request,
		Facts:   params.Facts,
	}
	if params.IgnoreRemembered {
		evaluate.Forget = h.Forget
	}
	form, err := kernel.Evaluate(evaluate)
	if err != nil {
		return kernel.Form[Req]{}, classify(err)
	}
	return form, nil
}

type ChoicesParams[Req, F any] struct {
	Path    string
	Request Req
	Facts   F
}

func (h *Handle[Req, F, P, D]) Choices(ctx context.Context, params ChoicesParams[Req, F]) (kernel.Choices, error) {
	index := slices.IndexFunc(h.command.Fields, func(field kernel.FieldDef[Req, F]) bool { return field.Spec.Path == params.Path })
	if index < 0 {
		return kernel.Choices{}, kernel.NotFound(kernel.Problem{Code: kernel.CodeUnknownField, Params: kernel.Params{kernel.ParamPath: params.Path}})
	}
	field := h.command.Fields[index]
	if field.Choices == nil {
		return kernel.Choices{}, nil
	}
	choices, err := field.Choices(ctx, params.Request, params.Facts)
	if err != nil {
		return kernel.Choices{}, classify(err)
	}
	current, err := kernel.Get(params.Request, params.Path)
	if err != nil {
		return kernel.Choices{}, classify(err)
	}
	options := slices.Clone(choices.Options)
	for i, option := range options {
		if option.Refusal != nil {
			continue
		}
		picked, judged, err := candidate(candidateParams{Type: field.Spec.Type, Current: current, Option: option.Value})
		if err != nil {
			return kernel.Choices{}, classify(err)
		}
		if !judged {
			continue
		}
		refusal, err := kernel.Disabled(kernel.DisabledParams[Req, F]{
			Rules:     h.command.Rules,
			Request:   params.Request,
			Facts:     params.Facts,
			Path:      params.Path,
			Candidate: picked,
		})
		if err != nil {
			return kernel.Choices{}, classify(err)
		}
		options[i].Refusal = refusal
	}
	choices.Options = options
	return choices, nil
}

type candidateParams struct {
	Type    kernel.FieldType
	Current kernel.Value
	Option  string
}

// candidate is the field's value once the option is picked: added to what is
// already chosen for a list, so a rule over the list sees the whole of it. An
// option of a decisions field is a choice for one key among several: it is
// not judged alone.
func candidate(params candidateParams) (kernel.Value, bool, error) {
	switch params.Type {
	case kernel.FieldMultiSelect, kernel.FieldReorder, kernel.FieldTextList:
		chosen, ok := params.Current.(kernel.List)
		if !ok && params.Current != nil {
			return nil, false, fmt.Errorf(candidateShapeFmt, params.Type, params.Current)
		}
		return append(slices.Clone(chosen), params.Option), true, nil
	case kernel.FieldBool:
		picked, err := strconv.ParseBool(params.Option)
		return kernel.Bool(picked), err == nil, err
	case kernel.FieldDecisions:
		return nil, false, nil
	}
	return kernel.Text(params.Option), true, nil
}
