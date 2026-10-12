package dispatch

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const planShapeFmt = "%s: the detail of a plan is a struct, not %s"

type Handle[Req, F, P, D any] struct {
	dispatcher *Dispatcher
	command    kernel.Command[Req, F, P, D]
}

type RegisterParams[Req, F, P, D any] struct {
	Dispatcher *Dispatcher
	Command    kernel.Command[Req, F, P, D]
}

// Register panics on a plan whose detail is not a struct: every surface shows
// a plan as its fields, so a plan that has none could not be shown.
func Register[Req, F, P, D any](params RegisterParams[Req, F, P, D]) *Handle[Req, F, P, D] {
	if detail := reflect.TypeFor[P](); detail.Kind() != reflect.Struct {
		panic(fmt.Sprintf(planShapeFmt, params.Command.Name, detail))
	}
	return &Handle[Req, F, P, D]{dispatcher: params.Dispatcher, command: params.Command}
}

func (h *Handle[Req, F, P, D]) Name() string { return h.command.Name }

func (h *Handle[Req, F, P, D]) Schema() []kernel.FieldSpec {
	specs := make([]kernel.FieldSpec, 0, len(h.command.Fields))
	for _, field := range h.command.Fields {
		specs = append(specs, field.Spec)
	}
	return specs
}

type Snapshot[F any] struct {
	Facts       F
	Fingerprint Fingerprint
}

func (h *Handle[Req, F, P, D]) Observe(ctx context.Context) (Snapshot[F], error) {
	if h.command.Observe == nil {
		var none F
		return Snapshot[F]{Facts: none, Fingerprint: fingerprintOf(none)}, nil
	}
	facts, err := h.command.Observe(ctx, kernel.ObserveScope{Dir: h.dispatcher.params.Dir})
	if err != nil {
		return Snapshot[F]{}, classify(err)
	}
	return Snapshot[F]{Facts: facts, Fingerprint: fingerprintOf(facts)}, nil
}

func (h *Handle[Req, F, P, D]) Forget(facts F) F {
	if h.command.Forget == nil {
		return facts
	}
	return h.command.Forget(facts)
}

type FormParams[Req, F any] struct {
	Request Req
	Facts   F
}

func (h *Handle[Req, F, P, D]) State(params FormParams[Req, F]) (kernel.Form[Req], error) {
	form, err := kernel.Evaluate(kernel.EvaluateParams[Req, F]{
		Fields:  h.command.Fields,
		Rules:   h.command.Rules,
		Request: params.Request,
		Facts:   params.Facts,
	})
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
		return kernel.Choices{}, kernel.NotFound(kernel.Problem{Code: kernel.CodeNotFound, Params: kernel.Params{kernel.ParamPath: params.Path, kernel.ParamValue: params.Path}})
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
		refusal, err := kernel.Disabled(kernel.DisabledParams[Req, F]{
			Rules:     h.command.Rules,
			Request:   params.Request,
			Facts:     params.Facts,
			Path:      params.Path,
			Candidate: candidate(candidateParams{Type: field.Spec.Type, Current: current, Option: option.Value}),
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
// already chosen for a list, so a rule over the list sees the whole of it.
func candidate(params candidateParams) kernel.Value {
	switch params.Type {
	case kernel.FieldMultiSelect, kernel.FieldReorder, kernel.FieldTextList:
		chosen, _ := params.Current.(kernel.List)
		return append(slices.Clone(chosen), params.Option)
	}
	return kernel.Text(params.Option)
}
