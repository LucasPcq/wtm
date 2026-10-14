package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const (
	decisionSeparator     = "="
	pathSeparator         = "."
	unexpectedArgumentFmt = "unexpected argument %q: the command takes none"
)

type entry struct {
	Path  string
	Value any
}

// request is the JSON request the command line gives: a flag only when it was
// passed, so that a default stays dispatch's to decide.
func (b *binding) request(cmd *cobra.Command, args []string) (json.RawMessage, kernel.Error) {
	if len(args) > 0 && b.positional == "" {
		invalid := kernel.Invalid(nil)
		invalid.Cause = fmt.Errorf(unexpectedArgumentFmt, args[0])
		return nil, invalid
	}
	entries, problems, err := b.entries(cmd, args)
	if err != nil {
		return nil, kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: err})
	}
	if len(problems) > 0 {
		return nil, kernel.Invalid(problems)
	}
	data, err := json.Marshal(nested(entries))
	if err != nil {
		return nil, kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: err})
	}
	return data, nil
}

// entries reads a flag only through the getter of the type Bind gave it: an
// error there is a flag bound by hand against its field.
func (b *binding) entries(cmd *cobra.Command, args []string) ([]entry, []kernel.FieldError, error) {
	var entries []entry
	var problems []kernel.FieldError
	for _, spec := range b.specs {
		read := b.value(valueParams{Cmd: cmd, Spec: spec, Args: args})
		if read.Err != nil {
			return nil, nil, read.Err
		}
		if read.Problem != nil {
			problems = append(problems, *read.Problem)
		}
		if read.Given {
			entries = append(entries, entry{Path: spec.Path, Value: read.Value})
		}
	}
	return entries, problems, nil
}

type valueParams struct {
	Cmd  *cobra.Command
	Spec kernel.FieldSpec
	Args []string
}

type valueRead struct {
	Value   any
	Given   bool
	Problem *kernel.FieldError
	Err     error
}

func (b *binding) value(params valueParams) valueRead {
	if params.Spec.Path == b.positional {
		return positionalValue(params.Spec, params.Args)
	}
	name := b.flagOf(params.Spec.Path).Name
	if !params.Cmd.Flags().Changed(name) {
		return valueRead{}
	}
	flags := params.Cmd.Flags()
	switch params.Spec.Type {
	case kernel.FieldBool:
		value, err := flags.GetBool(name)
		return valueRead{Value: value, Given: true, Err: err}
	case kernel.FieldTextList, kernel.FieldMultiSelect, kernel.FieldReorder:
		value, err := flags.GetStringArray(name)
		return valueRead{Value: value, Given: true, Err: err}
	case kernel.FieldDecisions:
		value, err := flags.GetStringArray(name)
		if err != nil {
			return valueRead{Err: err}
		}
		return decisions(params.Spec.Path, value)
	default:
		value, err := flags.GetString(name)
		return valueRead{Value: value, Given: true, Err: err}
	}
}

func positionalValue(spec kernel.FieldSpec, args []string) valueRead {
	if len(args) == 0 {
		return valueRead{}
	}
	switch spec.Type {
	case kernel.FieldTextList, kernel.FieldMultiSelect, kernel.FieldReorder:
		return valueRead{Value: args, Given: true}
	}
	if len(args) > 1 {
		return valueRead{Problem: &kernel.FieldError{Path: spec.Path, Code: kernel.CodeTooMany, Params: kernel.Params{kernel.ParamLimit: strconv.Itoa(1)}}}
	}
	return valueRead{Value: args[0], Given: true}
}

// decisions reads KEY=VALUE pairs; a pair with no key is refused on its field.
func decisions(path string, pairs []string) valueRead {
	chosen := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, decisionSeparator)
		if !found || key == "" {
			return valueRead{Problem: &kernel.FieldError{Path: path, Code: kernel.CodeInvalid, Params: kernel.Params{kernel.ParamValue: pair}}}
		}
		chosen[key] = value
	}
	return valueRead{Value: chosen, Given: true}
}

// nested turns dotted paths into the objects the request's JSON names.
func nested(entries []entry) map[string]any {
	root := map[string]any{}
	for _, entry := range entries {
		put(root, strings.Split(entry.Path, pathSeparator), entry.Value)
	}
	return root
}

func put(object map[string]any, path []string, value any) {
	if len(path) == 1 {
		object[path[0]] = value
		return
	}
	put(childOf(object, path[0]), path[1:], value)
}

func childOf(object map[string]any, key string) map[string]any {
	if child, ok := object[key].(map[string]any); ok {
		return child
	}
	child := map[string]any{}
	object[key] = child
	return child
}
