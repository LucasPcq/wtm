package kernel

import (
	"reflect"
	"slices"
)

// Rule is a rule across fields, kept as data: the form reads Paths to know
// which options a rule may disable.
type Rule[Req, F any] struct {
	Code  Code
	Paths []string
	Check func(Req, F) ([]FieldError, error)
}

// Rules is built by chaining combinators, the types given once:
// kernel.Rules[Request, Facts]{}.Distinct("branches").RequiredWhen(…).
type Rules[Req, F any] []Rule[Req, F]

// Condition is a pure predicate over the request and the facts; Code says
// why it holds, for the message.
type Condition[Req, F any] struct {
	Code  Code
	Holds func(Req, F) bool
}

type RequiresRule struct {
	Field string
	Needs []string
}

type SelfParentRule struct {
	Parent   string
	Children string
}

// reading is what a combinator checks: the values at its paths, in order.
type reading[Req, F any] struct {
	values  []Value
	request Req
	facts   F
}

func (rules Rules[Req, F]) With(rule Rule[Req, F]) Rules[Req, F] {
	return append(slices.Clip(rules), rule)
}

// reads adds a rule that checks the values at paths, read for it.
func (rules Rules[Req, F]) reads(code Code, paths []string, check func(reading[Req, F]) []FieldError) Rules[Req, F] {
	return rules.With(Rule[Req, F]{Code: code, Paths: paths, Check: func(req Req, facts F) ([]FieldError, error) {
		values := make([]Value, 0, len(paths))
		for _, path := range paths {
			value, err := Get(req, path)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return check(reading[Req, F]{values: values, request: req, facts: facts}), nil
	}})
}

// Exclusive refuses more than one of paths set.
func (rules Rules[Req, F]) Exclusive(paths ...string) Rules[Req, F] {
	return rules.reads(CodeExclusiveWith, paths, func(read reading[Req, F]) []FieldError {
		return exclusive(setAmong(paths, read.values))
	})
}

// OneOf wants exactly one of paths set.
func (rules Rules[Req, F]) OneOf(paths ...string) Rules[Req, F] {
	return rules.reads(CodeRequiredOneOf, paths, func(read reading[Req, F]) []FieldError {
		set := setAmong(paths, read.values)
		if len(set) > 0 {
			return exclusive(set)
		}
		return []FieldError{{Path: paths[0], Code: CodeRequiredOneOf, With: paths[1:]}}
	})
}

func (rules Rules[Req, F]) Requires(rule RequiresRule) Rules[Req, F] {
	paths := append([]string{rule.Field}, rule.Needs...)
	return rules.reads(CodeRequires, paths, func(read reading[Req, F]) []FieldError {
		set := setAmong(paths, read.values)
		if !slices.Contains(set, rule.Field) {
			return nil
		}
		missing := without(rule.Needs, func(need string) bool { return slices.Contains(set, need) })
		if len(missing) == 0 {
			return nil
		}
		return []FieldError{{Path: rule.Field, Code: CodeRequires, With: missing}}
	})
}

func (rules Rules[Req, F]) RequiredWhen(path string, when Condition[Req, F]) Rules[Req, F] {
	return rules.reads(CodeRequired, []string{path}, func(read reading[Req, F]) []FieldError {
		if !IsZero(read.values[0]) || !when.Holds(read.request, read.facts) {
			return nil
		}
		return []FieldError{{Path: path, Code: CodeRequired, Params: Params{ParamBecause: string(when.Code)}}}
	})
}

// Distinct refuses an entry given twice in a list.
func (rules Rules[Req, F]) Distinct(path string) Rules[Req, F] {
	return rules.reads(CodeDistinct, []string{path}, func(read reading[Req, F]) []FieldError {
		list, _ := read.values[0].(List)
		var problems []FieldError
		for index, entry := range list {
			if slices.Contains(list[:index], entry) {
				problems = append(problems, FieldError{Path: IndexPath(path, index), Code: CodeDistinct, Params: Params{ParamValue: entry}})
			}
		}
		return problems
	})
}

// NotSelfParent refuses a parent that is also one of the children it would
// be the parent of; Children is a list or a single text field.
func (rules Rules[Req, F]) NotSelfParent(rule SelfParentRule) Rules[Req, F] {
	return rules.reads(CodeSelfParent, []string{rule.Parent, rule.Children}, func(read reading[Req, F]) []FieldError {
		parent, _ := read.values[0].(Text)
		if parent == "" || !holds(read.values[1], string(parent)) {
			return nil
		}
		return []FieldError{{Path: rule.Parent, Code: CodeSelfParent, Params: Params{ParamValue: string(parent)}, With: []string{rule.Children}}}
	})
}

type CheckRulesParams[Req, F any] struct {
	Rules   Rules[Req, F]
	Request Req
	Facts   F
}

func CheckRules[Req, F any](params CheckRulesParams[Req, F]) ([]FieldError, error) {
	var problems []FieldError
	for _, rule := range params.Rules {
		found, err := rule.Check(params.Request, params.Facts)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

type DisabledParams[Req, F any] struct {
	Rules     Rules[Req, F]
	Request   Req
	Facts     F
	Path      string
	Candidate Value
}

// Disabled says whether picking Candidate for Path would break a rule that
// holds today, and which: the form shows the option disabled with it.
func Disabled[Req, F any](params DisabledParams[Req, F]) (*FieldError, error) {
	touching := Rules[Req, F](without(params.Rules, func(rule Rule[Req, F]) bool {
		return !slices.Contains(rule.Paths, params.Path)
	}))
	before, err := CheckRules(CheckRulesParams[Req, F]{Rules: touching, Request: params.Request, Facts: params.Facts})
	if err != nil {
		return nil, err
	}
	picked, err := Set(params.Request, params.Path, params.Candidate)
	if err != nil {
		return nil, err
	}
	after, err := CheckRules(CheckRulesParams[Req, F]{Rules: touching, Request: picked, Facts: params.Facts})
	if err != nil {
		return nil, err
	}
	for _, problem := range after {
		if !slices.ContainsFunc(before, func(known FieldError) bool { return reflect.DeepEqual(known, problem) }) {
			return &problem, nil
		}
	}
	return nil, nil
}

func setAmong(paths []string, values []Value) []string {
	var set []string
	for index, path := range paths {
		if !IsZero(values[index]) {
			set = append(set, path)
		}
	}
	return set
}

func exclusive(set []string) []FieldError {
	if len(set) < 2 {
		return nil
	}
	problems := make([]FieldError, 0, len(set)-1)
	for _, path := range set[1:] {
		others := without(set, func(other string) bool { return other == path })
		problems = append(problems, FieldError{Path: path, Code: CodeExclusiveWith, With: others})
	}
	return problems
}

// holds says whether a list or a text field holds text.
func holds(value Value, text string) bool {
	switch v := value.(type) {
	case List:
		return slices.Contains(v, text)
	case Text:
		return string(v) == text
	case Bool, Decisions, nil:
		return false
	}
	return false
}

// without returns a copy of items with those drop matches left out.
func without[T any](items []T, drop func(T) bool) []T {
	return slices.DeleteFunc(slices.Clone(items), drop)
}
