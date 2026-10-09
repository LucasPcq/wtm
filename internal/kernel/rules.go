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

func (rules Rules[Req, F]) With(rule Rule[Req, F]) Rules[Req, F] {
	return append(slices.Clip(rules), rule)
}

// Exclusive refuses more than one of paths set.
func (rules Rules[Req, F]) Exclusive(paths ...string) Rules[Req, F] {
	return rules.With(Rule[Req, F]{Code: CodeExclusiveWith, Paths: paths, Check: func(req Req, _ F) ([]FieldError, error) {
		set, err := setAmong(req, paths)
		return exclusive(set), err
	}})
}

// OneOf wants exactly one of paths set.
func (rules Rules[Req, F]) OneOf(paths ...string) Rules[Req, F] {
	return rules.With(Rule[Req, F]{Code: CodeRequired, Paths: paths, Check: func(req Req, _ F) ([]FieldError, error) {
		set, err := setAmong(req, paths)
		if err != nil || len(set) > 0 {
			return exclusive(set), err
		}
		return []FieldError{{Path: paths[0], Code: CodeRequired, With: paths[1:]}}, nil
	}})
}

func (rules Rules[Req, F]) Requires(rule RequiresRule) Rules[Req, F] {
	paths := append([]string{rule.Field}, rule.Needs...)
	return rules.With(Rule[Req, F]{Code: CodeRequires, Paths: paths, Check: func(req Req, _ F) ([]FieldError, error) {
		set, err := setAmong(req, paths)
		if err != nil || !slices.Contains(set, rule.Field) {
			return nil, err
		}
		missing := slices.DeleteFunc(slices.Clone(rule.Needs), func(need string) bool { return slices.Contains(set, need) })
		if len(missing) == 0 {
			return nil, nil
		}
		return []FieldError{{Path: rule.Field, Code: CodeRequires, With: missing}}, nil
	}})
}

func (rules Rules[Req, F]) RequiredWhen(path string, when Condition[Req, F]) Rules[Req, F] {
	return rules.With(Rule[Req, F]{Code: CodeRequired, Paths: []string{path}, Check: func(req Req, facts F) ([]FieldError, error) {
		value, err := Get(req, path)
		if err != nil || !IsZero(value) || !when.Holds(req, facts) {
			return nil, err
		}
		return []FieldError{{Path: path, Code: CodeRequired, Params: map[string]string{ParamBecause: string(when.Code)}}}, nil
	}})
}

// Distinct refuses an entry given twice in a list.
func (rules Rules[Req, F]) Distinct(path string) Rules[Req, F] {
	return rules.With(Rule[Req, F]{Code: CodeDistinct, Paths: []string{path}, Check: func(req Req, _ F) ([]FieldError, error) {
		value, err := Get(req, path)
		if err != nil {
			return nil, err
		}
		var problems []FieldError
		for index, entry := range value.List {
			if slices.Contains(value.List[:index], entry) {
				problems = append(problems, FieldError{Path: IndexPath(path, index), Code: CodeDistinct, Params: map[string]string{ParamValue: entry}})
			}
		}
		return problems, nil
	}})
}

// NotSelfParent refuses a parent that is also one of the children it would
// be the parent of.
func (rules Rules[Req, F]) NotSelfParent(rule SelfParentRule) Rules[Req, F] {
	return rules.With(Rule[Req, F]{Code: CodeSelfParent, Paths: []string{rule.Parent, rule.Children}, Check: func(req Req, _ F) ([]FieldError, error) {
		parent, err := Get(req, rule.Parent)
		if err != nil || parent.Text == "" {
			return nil, err
		}
		children, err := Get(req, rule.Children)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(children.List, parent.Text) && children.Text != parent.Text {
			return nil, nil
		}
		return []FieldError{{Path: rule.Parent, Code: CodeSelfParent, Params: map[string]string{ParamValue: parent.Text}, With: []string{rule.Children}}}, nil
	}})
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
	touching := Rules[Req, F](slices.DeleteFunc(slices.Clone(params.Rules), func(rule Rule[Req, F]) bool {
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

func setAmong[Req any](req Req, paths []string) ([]string, error) {
	var set []string
	for _, path := range paths {
		value, err := Get(req, path)
		if err != nil {
			return nil, err
		}
		if !IsZero(value) {
			set = append(set, path)
		}
	}
	return set, nil
}

func exclusive(set []string) []FieldError {
	if len(set) < 2 {
		return nil
	}
	problems := make([]FieldError, 0, len(set)-1)
	for _, path := range set[1:] {
		others := slices.DeleteFunc(slices.Clone(set), func(other string) bool { return other == path })
		problems = append(problems, FieldError{Path: path, Code: CodeExclusiveWith, With: others})
	}
	return problems
}
