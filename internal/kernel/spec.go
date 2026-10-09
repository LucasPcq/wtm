package kernel

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

type CheckSpecParams[Req any] struct {
	Request Req
	Specs   []FieldSpec
}

// CheckSpec checks each field against its own declared constraints; rules
// across fields are Rules' business. A spec whose type does not fit its
// field, or whose pattern does not compile, is an error of the command.
func CheckSpec[Req any](params CheckSpecParams[Req]) ([]FieldError, error) {
	var problems []FieldError
	for _, spec := range params.Specs {
		value, err := Get(params.Request, spec.Path)
		if err != nil {
			return nil, err
		}
		check, err := checkerOf(spec, value)
		if err != nil {
			return nil, err
		}
		problems = append(problems, check.field(value)...)
	}
	return problems, nil
}

// fits says which Value a field type holds.
func fits(typ FieldType, value Value) bool {
	switch value.(type) {
	case Text:
		return typ == FieldText || typ == FieldSelect
	case Bool:
		return typ == FieldBool
	case List:
		return typ == FieldMultiSelect || typ == FieldReorder || typ == FieldTextList
	case Decisions:
		return typ == FieldDecisions
	}
	return false
}

type checker struct {
	spec    FieldSpec
	pattern *regexp.Regexp
}

type entry struct {
	path string
	text string
}

func checkerOf(spec FieldSpec, value Value) (checker, error) {
	if !fits(spec.Type, value) {
		return checker{}, fmt.Errorf("kernel: field %q: a %s field cannot hold a %T", spec.Path, spec.Type, value)
	}
	if spec.Constraints.Pattern == "" {
		return checker{spec: spec}, nil
	}
	pattern, err := regexp.Compile(spec.Constraints.Pattern)
	if err != nil {
		return checker{}, fmt.Errorf("kernel: field %q: pattern: %w", spec.Path, err)
	}
	return checker{spec: spec, pattern: pattern}, nil
}

func (c checker) field(value Value) []FieldError {
	if IsZero(value) && c.spec.Required {
		return []FieldError{{Path: c.spec.Path, Code: CodeRequired}}
	}
	if IsZero(value) {
		return nil
	}
	switch v := value.(type) {
	case Text:
		return c.entry(entry{path: c.spec.Path, text: string(v)})
	case List:
		return c.list(v)
	case Decisions:
		return c.decisions(v)
	case Bool:
		return nil
	}
	return nil
}

func (c checker) list(list List) []FieldError {
	limits := c.spec.Constraints
	if limits.MinItems > 0 && len(list) < limits.MinItems {
		return []FieldError{{Path: c.spec.Path, Code: CodeTooFew, Params: Params{ParamLimit: strconv.Itoa(limits.MinItems)}}}
	}
	if limits.MaxItems > 0 && len(list) > limits.MaxItems {
		return []FieldError{{Path: c.spec.Path, Code: CodeTooMany, Params: Params{ParamLimit: strconv.Itoa(limits.MaxItems)}}}
	}
	var problems []FieldError
	for index, text := range list {
		problems = append(problems, c.entry(entry{path: IndexPath(c.spec.Path, index), text: text})...)
	}
	return problems
}

func (c checker) decisions(decisions Decisions) []FieldError {
	keys := make([]string, 0, len(decisions))
	for key := range decisions {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var problems []FieldError
	for _, key := range keys {
		problems = append(problems, c.entry(entry{path: c.spec.Path + "[" + key + "]", text: decisions[key]})...)
	}
	return problems
}

func (c checker) entry(at entry) []FieldError {
	limits := c.spec.Constraints
	if len(limits.Enum) > 0 && !slices.Contains(limits.Enum, at.text) {
		return []FieldError{{Path: at.path, Code: CodeOneOf, Params: Params{ParamValue: at.text}, Accepted: limits.Enum}}
	}
	if limits.MinLen > 0 && len(at.text) < limits.MinLen {
		return []FieldError{{Path: at.path, Code: CodeTooShort, Params: Params{ParamLimit: strconv.Itoa(limits.MinLen)}}}
	}
	if limits.MaxLen > 0 && len(at.text) > limits.MaxLen {
		return []FieldError{{Path: at.path, Code: CodeTooLong, Params: Params{ParamLimit: strconv.Itoa(limits.MaxLen)}}}
	}
	if c.pattern != nil && !c.pattern.MatchString(at.text) {
		return []FieldError{{Path: at.path, Code: CodePattern, Params: Params{ParamValue: at.text, ParamPattern: limits.Pattern}}}
	}
	return nil
}

func IndexPath(path string, index int) string {
	return path + "[" + strconv.Itoa(index) + "]"
}
