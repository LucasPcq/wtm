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
// across fields are Rules' business.
func CheckSpec[Req any](params CheckSpecParams[Req]) ([]FieldError, error) {
	var problems []FieldError
	for _, spec := range params.Specs {
		value, err := Get(params.Request, spec.Path)
		if err != nil {
			return nil, err
		}
		check, err := checkerOf(spec)
		if err != nil {
			return nil, err
		}
		problems = append(problems, check.field(value)...)
	}
	return problems, nil
}

type checker struct {
	spec    FieldSpec
	pattern *regexp.Regexp
}

type entry struct {
	path string
	text string
}

func checkerOf(spec FieldSpec) (checker, error) {
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
	switch c.spec.Type {
	case FieldMultiSelect, FieldReorder, FieldTextList:
		return c.list(value.List)
	case FieldDecisions:
		return c.decisions(value.Decisions)
	case FieldBool:
		return nil
	}
	return c.entry(entry{path: c.spec.Path, text: value.Text})
}

func (c checker) list(list []string) []FieldError {
	limits := c.spec.Constraints
	if limits.MinItems > 0 && len(list) < limits.MinItems {
		return []FieldError{{Path: c.spec.Path, Code: CodeTooFew, Params: limitParam(limits.MinItems)}}
	}
	if limits.MaxItems > 0 && len(list) > limits.MaxItems {
		return []FieldError{{Path: c.spec.Path, Code: CodeTooMany, Params: limitParam(limits.MaxItems)}}
	}
	var problems []FieldError
	for index, text := range list {
		problems = append(problems, c.entry(entry{path: IndexPath(c.spec.Path, index), text: text})...)
	}
	return problems
}

func (c checker) decisions(decisions map[string]string) []FieldError {
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
		return []FieldError{{Path: at.path, Code: CodeOneOf, Params: map[string]string{ParamValue: at.text}, Accepted: limits.Enum}}
	}
	if limits.MinLen > 0 && len(at.text) < limits.MinLen {
		return []FieldError{{Path: at.path, Code: CodeTooShort, Params: limitParam(limits.MinLen)}}
	}
	if limits.MaxLen > 0 && len(at.text) > limits.MaxLen {
		return []FieldError{{Path: at.path, Code: CodeTooLong, Params: limitParam(limits.MaxLen)}}
	}
	if c.pattern != nil && !c.pattern.MatchString(at.text) {
		return []FieldError{{Path: at.path, Code: CodePattern, Params: map[string]string{ParamValue: at.text, ParamPattern: limits.Pattern}}}
	}
	return nil
}

func limitParam(limit int) map[string]string {
	return map[string]string{ParamLimit: strconv.Itoa(limit)}
}

func IndexPath(path string, index int) string {
	return path + "[" + strconv.Itoa(index) + "]"
}
