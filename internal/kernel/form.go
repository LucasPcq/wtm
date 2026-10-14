package kernel

import (
	"encoding/json"
	"slices"
)

type Origin string

const (
	OriginRequest    Origin = "request"
	OriginRemembered Origin = "remembered"
	OriginConfig     Origin = "config"
	OriginDefault    Origin = "default"
)

type Fallback struct {
	Value  Value
	Origin Origin
}

type FieldStatus string

const (
	FieldMissing   FieldStatus = "missing"
	FieldProvided  FieldStatus = "provided"
	FieldDefaulted FieldStatus = "defaulted"
	FieldSkipped   FieldStatus = "skipped"
)

type FieldState struct {
	Path       string       `json:"path"`
	Status     FieldStatus  `json:"status"`
	Origin     Origin       `json:"origin,omitempty"`
	Value      Value        `json:"value"`
	Remembered Value        `json:"remembered,omitempty"`
	SkipReason Code         `json:"skip_reason,omitempty"`
	Errors     []FieldError `json:"errors,omitempty"`
}

func (s *FieldState) UnmarshalJSON(data []byte) error {
	type plain FieldState
	var decoded struct {
		plain
		Value      json.RawMessage `json:"value"`
		Remembered json.RawMessage `json:"remembered"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	value, err := decodeValue(decoded.Value)
	if err != nil {
		return err
	}
	remembered, err := decodeValue(decoded.Remembered)
	if err != nil {
		return err
	}
	*s = FieldState(decoded.plain)
	s.Value, s.Remembered = value, remembered
	return nil
}

type EvaluateParams[Req, F any] struct {
	Fields  []FieldDef[Req, F]
	Rules   Rules[Req, F]
	Request Req
	Facts   F
	Forget  func(F) F
}

type Form[Req any] struct {
	Request  Req          `json:"request"`
	States   []FieldState `json:"states"`
	Errors   []FieldError `json:"errors,omitempty"`
	Complete bool         `json:"complete"`
}

func Evaluate[Req, F any](params EvaluateParams[Req, F]) (Form[Req], error) {
	request := params.Request
	facts := params.Facts
	if params.Forget != nil {
		facts = params.Forget(params.Facts)
	}
	states := make([]FieldState, 0, len(params.Fields))
	var problems []FieldError
	for _, field := range params.Fields {
		state, filled, err := settle(settleParams[Req, F]{Field: field, Request: request, Facts: facts})
		if err != nil {
			return Form[Req]{}, err
		}
		if params.Forget != nil {
			state.Remembered = remembered(settleParams[Req, F]{Field: field, Request: request, Facts: params.Facts})
		}
		request = filled
		found, err := fieldProblems(fieldProblemsParams[Req, F]{Field: field, State: state, Request: request, Facts: facts})
		if err != nil {
			return Form[Req]{}, err
		}
		state.Errors = found
		states = append(states, state)
		problems = append(problems, found...)
	}
	crossed, err := CheckRules(CheckRulesParams[Req, F]{Rules: params.Rules, Request: request, Facts: facts})
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
	fallback, ok := defaulted(params)
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

func remembered[Req, F any](params settleParams[Req, F]) Value {
	fallback, ok := defaulted(params)
	if !ok || fallback.Origin != OriginRemembered {
		return nil
	}
	return fallback.Value
}

func defaulted[Req, F any](params settleParams[Req, F]) (Fallback, bool) {
	if params.Field.Default == nil {
		return Fallback{}, false
	}
	return params.Field.Default(params.Request, params.Facts)
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
		out[index].Errors = append(slices.Clip(state.Errors), without(crossed, func(problem FieldError) bool {
			return !Within(PathIn{Path: problem.Path, Parent: state.Path})
		})...)
	}
	return out
}
