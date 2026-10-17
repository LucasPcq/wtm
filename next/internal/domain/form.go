package domain

import (
	"cmp"
	"encoding/json"
	"strconv"
	"strings"
)

// Form is a command's request as a schema a surface renders without knowing the command: its fields in the order a wizard asks them, the request's flags and its --help spelled from them, and the request's problems, each naming a field, an entry of one, or a field the form does not list (remember[0]).
type Form struct {
	Fields   []Field
	Problems []FieldError
}

// MarshalJSON is the form as a surface receives it: each field with its "kind" (select, multi_select, text, text_list, confirm) first, then what that kind carries, and the request's problems.
func (f Form) MarshalJSON() ([]byte, error) {
	fields := make([]json.RawMessage, 0, len(f.Fields))
	for _, field := range f.Fields {
		kind, listed := fieldKind(field)
		raw, err := discriminated("kind", kind, listed)
		if err != nil {
			return nil, err
		}
		fields = append(fields, raw)
	}

	return encoded(struct {
		Fields   []json.RawMessage `json:"fields"`
		Problems []FieldError      `json:"problems"`
	}{fields, orEmpty(f.Problems)})
}

// fieldKind returns the kind of field and the field with each of its lists an array, never null, so a surface iterates them as they are.
func fieldKind(field Field) (string, Field) {
	switch field := field.(type) {
	case SelectField:
		field.Options = orEmpty(field.Options)
		return "select", field
	case MultiSelectField:
		field.Picked, field.Options = orEmpty(field.Picked), orEmpty(field.Options)
		return "multi_select", field
	case TextField:
		return "text", field
	case TextListField:
		field.Entries, field.Options = orEmpty(field.Entries), orEmpty(field.Options)
		return "text_list", field
	case ConfirmField:
		return "confirm", field
	}
	return "", field
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// PathRemember is the path of the fields whose answer is remembered: the contract's --remember, accepting the paths of the form's rememberable fields.
const PathRemember = "remember"

//go-sumtype:decl Field

// Field is one field of a Form, of one kind: SelectField, MultiSelectField, TextField, TextListField or ConfirmField, each carrying only what is valid for it.
type Field interface {
	// Spec is what a field of every kind has.
	Spec() FieldSpec
	// Answer is the field's answer as the values its command's Request.Answer takes.
	Answer() []string
	field()
}

// FieldSpec is what a field of every kind has.
type FieldSpec struct {
	Path   string `json:"path"`             // the request field it answers, as a field error names it
	Flag   string `json:"flag"`             // the argument or flag that sets it: <branches>, --from
	Label  string `json:"label"`            // its name in a wizard's breadcrumb, trail and problems
	Help   string `json:"help"`             // what its answer decides, as a wizard describes it
	Usage  string `json:"usage,omitempty"`  // the usage of its flag, when it is not Help
	Origin Origin `json:"origin,omitempty"` // where its answer came from, empty when the field does not say
	Skip   string `json:"skip,omitempty"`   // why it is not asked, empty when it is
}

// Spec is the spec itself, so each kind of field has it by embedding.
func (s FieldSpec) Spec() FieldSpec {
	return s
}

// SelectField asks one of its options; Pinned is the option listed first, Remembered the answer remembered for it, Remember whether this run's answer is to be.
type SelectField struct {
	FieldSpec
	Value        string   `json:"value"`
	Options      []Option `json:"options"`
	Pinned       string   `json:"pinned,omitempty"`
	Remembered   string   `json:"remembered,omitempty"`
	Rememberable bool     `json:"rememberable"`
	Remember     bool     `json:"remember"`
}

// MultiSelectField asks any of its options.
type MultiSelectField struct {
	FieldSpec
	Picked  []string `json:"picked"`
	Options []Option `json:"options"`
}

// TextField asks one line of text.
type TextField struct {
	FieldSpec
	Value string `json:"value"`
}

// TextListField asks a list of entries; Options are the values the repository knows, each with its badge.
type TextListField struct {
	FieldSpec
	Entries []string `json:"entries"`
	Options []Option `json:"options"`
}

// ConfirmField asks yes or no; Yes and No are its two options, valued true and false, with how each reads.
type ConfirmField struct {
	FieldSpec
	Value        bool   `json:"value"`
	Yes          Option `json:"yes"`
	No           Option `json:"no"`
	Rememberable bool   `json:"rememberable"`
	Remember     bool   `json:"remember"`
}

// Answer is the option selected.
func (f SelectField) Answer() []string { return []string{f.Value} }

// Answer is the options picked.
func (f MultiSelectField) Answer() []string { return f.Picked }

// Answer is the text typed.
func (f TextField) Answer() []string { return []string{f.Value} }

// Answer is the entries.
func (f TextListField) Answer() []string { return f.Entries }

// Answer is true or false, as strconv spells it.
func (f ConfirmField) Answer() []string { return []string{strconv.FormatBool(f.Value)} }

func (SelectField) field()      {}
func (MultiSelectField) field() {}
func (TextField) field()        {}
func (TextListField) field()    {}
func (ConfirmField) field()     {}

// Flags spells each field path as the argument or flag that sets it, with the usage of that flag (its Usage, else its Help); the first field answering a path gives both. A form with a rememberable field also has --remember.
func (f Form) Flags() (flags, usages map[string]string) {
	flags, usages = map[string]string{}, map[string]string{}
	for _, field := range f.Fields {
		spec := field.Spec()
		if _, seen := flags[spec.Path]; !seen {
			flags[spec.Path], usages[spec.Path] = spec.Flag, cmp.Or(spec.Usage, spec.Help)
		}
	}

	if rememberable := f.Rememberable(); len(rememberable) > 0 {
		flags[PathRemember] = "--" + PathRemember
		usages[PathRemember] = "The answers to always use in this repository: " + strings.Join(rememberable, ", ") + " (repeatable)"
	}

	return flags, usages
}

// Rememberable are the paths of the fields whose answer can be remembered, in order.
func (f Form) Rememberable() []string {
	return f.paths(func(field Field) bool {
		rememberable, _ := remembers(field)
		return rememberable
	})
}

// Remembering are the paths of the fields whose answer is to be remembered, in order.
func (f Form) Remembering() []string {
	return f.paths(func(field Field) bool {
		_, remember := remembers(field)
		return remember
	})
}

func (f Form) paths(keep func(Field) bool) []string {
	var paths []string
	for _, field := range f.Fields {
		if keep(field) {
			paths = append(paths, field.Spec().Path)
		}
	}

	return paths
}

func remembers(field Field) (rememberable, remember bool) {
	switch field := field.(type) {
	case SelectField:
		return field.Rememberable, field.Remember
	case ConfirmField:
		return field.Rememberable, field.Remember
	case MultiSelectField, TextField, TextListField:
	}

	return false, false
}

// ProblemsOf are the problems that name the field at path or one of its entries.
func ProblemsOf(problems []FieldError, path string) []FieldError {
	var found []FieldError
	for _, problem := range problems {
		if problem.Path == path || strings.HasPrefix(problem.Path, path+"[") {
			found = append(found, problem)
		}
	}

	return found
}

// CodeUnknown is the field-error code of a path no field of a form answers.
const CodeUnknown = "unknown"

// Unknown is the problem of a path no field of a form answers, as an error of domain.Invalid.
func Unknown(path string) error {
	return Invalid([]FieldError{{Path: path, Code: CodeUnknown, Template: "{path} is not a field of this request"}})
}

// AnswerParams are the values given to the field at path.
type AnswerParams struct {
	Path   string
	Values []string
}

// CodeOneValue is the field-error code of a field given a count of values other than one.
const CodeOneValue = "one_value"

// One is the single value given to a field, else the problem of a count other than one.
func One[T ~string](params AnswerParams) (T, error) {
	if len(params.Values) != 1 {
		return "", Invalid([]FieldError{{Path: params.Path, Code: CodeOneValue, Template: "{path} takes one value, not {count}", Params: Params{"count": strconv.Itoa(len(params.Values))}}})
	}

	return T(params.Values[0]), nil
}

// Yes is the answer given to a ConfirmField, spelled as its Answer and its options spell it: true or false.
func Yes(params AnswerParams) (bool, error) {
	value, err := One[string](params)
	if err != nil {
		return false, err
	}

	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	return false, Invalid([]FieldError{{Path: params.Path, Code: CodeOneOf, Template: "{path} must be one of {accepted}, not {value}", Params: Params{"value": value}, Accepted: []string{"true", "false"}}})
}
