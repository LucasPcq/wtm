package kernel

import (
	"slices"
	"strings"
)

type EvaluateParams[Req, F any] struct {
	Fields  []FieldDef[Req, F]
	Rules   Rules[Req, F]
	Request Req
	Facts   F
}

// Form is a request with its defaults applied, the state of every field and
// what keeps it from running.
type Form[Req any] struct {
	Request  Req          `json:"request"`
	States   []FieldState `json:"states"`
	Errors   []FieldError `json:"errors,omitempty"`
	Complete bool         `json:"complete"`
}

// Evaluate fills in defaults field by field, in declaration order (a Skip
// reads the defaults of the fields before it), then validates the whole.
func Evaluate[Req, F any](params EvaluateParams[Req, F]) (Form[Req], error) {
	request := params.Request
	states := make([]FieldState, 0, len(params.Fields))
	var problems []FieldError
	for _, field := range params.Fields {
		state, filled, err := settle(settleParams[Req, F]{Field: field, Request: request, Facts: params.Facts})
		if err != nil {
			return Form[Req]{}, err
		}
		request = filled
		found, err := fieldProblems(fieldProblemsParams[Req, F]{Field: field, State: state, Request: request, Facts: params.Facts})
		if err != nil {
			return Form[Req]{}, err
		}
		state.Errors = found
		states = append(states, state)
		problems = append(problems, found...)
	}
	crossed, err := CheckRules(CheckRulesParams[Req, F]{Rules: params.Rules, Request: request, Facts: params.Facts})
	if err != nil {
		return Form[Req]{}, err
	}
	problems = append(problems, crossed...)
	return Form[Req]{Request: request, States: attach(states, crossed), Errors: problems, Complete: len(problems) == 0}, nil
}

type settleParams[Req, F any] struct {
	Field   FieldDef[Req, F]
	Request Req
	Facts   F
}

func settle[Req, F any](params settleParams[Req, F]) (FieldState, Req, error) {
	path := params.Field.Spec.Path
	value, err := Get(params.Request, path)
	if err != nil {
		return FieldState{}, params.Request, err
	}
	if skip, reason := skipped(params); skip {
		return FieldState{Path: path, Status: FieldSkipped, Value: value, SkipReason: reason}, params.Request, nil
	}
	if !IsZero(value) {
		return FieldState{Path: path, Status: FieldProvided, Origin: OriginRequest, Value: value}, params.Request, nil
	}
	if params.Field.Default == nil {
		return FieldState{Path: path, Status: FieldMissing}, params.Request, nil
	}
	fallback, ok := params.Field.Default(params.Request, params.Facts)
	if !ok {
		return FieldState{Path: path, Status: FieldMissing}, params.Request, nil
	}
	filled, err := Set(params.Request, path, fallback.Value)
	if err != nil {
		return FieldState{}, params.Request, err
	}
	return FieldState{Path: path, Status: FieldDefaulted, Origin: fallback.Origin, Value: fallback.Value}, filled, nil
}

func skipped[Req, F any](params settleParams[Req, F]) (bool, Code) {
	if params.Field.Skip == nil {
		return false, ""
	}
	return params.Field.Skip(params.Request, params.Facts)
}

type fieldProblemsParams[Req, F any] struct {
	Field   FieldDef[Req, F]
	State   FieldState
	Request Req
	Facts   F
}

func fieldProblems[Req, F any](params fieldProblemsParams[Req, F]) ([]FieldError, error) {
	if params.State.Status == FieldSkipped {
		return nil, nil
	}
	found, err := CheckSpec(CheckSpecParams[Req]{Request: params.Request, Specs: []FieldSpec{params.Field.Spec}})
	if err != nil || len(found) > 0 || params.Field.Validate == nil {
		return found, err
	}
	return params.Field.Validate(params.Request, params.Facts), nil
}

func attach(states []FieldState, crossed []FieldError) []FieldState {
	out := make([]FieldState, len(states))
	for index, state := range states {
		out[index] = state
		out[index].Errors = append(slices.Clip(state.Errors), slices.DeleteFunc(slices.Clone(crossed), func(problem FieldError) bool {
			return problem.Path != state.Path && !strings.HasPrefix(problem.Path, state.Path+"[")
		})...)
	}
	return out
}
