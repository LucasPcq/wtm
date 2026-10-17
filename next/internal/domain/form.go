package domain

import (
	"cmp"
	"strconv"
	"strings"
)

// Form is a command's request as a schema a surface renders without knowing the command: its fields in the order a wizard asks them, the request's flags and its --help spelled from them.
type Form struct {
	Fields []Field
}

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
	Path     string // the request field it answers, as a field error names it
	Flag     string // the argument or flag that sets it: <branches>, --from
	Label    string // its name in a wizard's breadcrumb, trail and problems
	Help     string // what its answer decides, as a wizard describes it
	Usage    string // the usage of its flag, when it is not Help
	Origin   Origin // where its answer came from, empty when the field does not say
	Skip     string // why it is not asked, empty when it is
	Problems []FieldError
}

// Spec is the spec itself, so each kind of field has it by embedding.
func (s FieldSpec) Spec() FieldSpec {
	return s
}

// SelectField asks one of its options; Pinned is the option listed first, Remembered the answer remembered for it, Remember whether this run's answer is to be.
type SelectField struct {
	FieldSpec
	Value        string
	Options      []Option
	Pinned       string
	Remembered   string
	Rememberable bool
	Remember     bool
}

// MultiSelectField asks any of its options.
type MultiSelectField struct {
	FieldSpec
	Picked  []string
	Options []Option
}

// TextField asks one line of text.
type TextField struct {
	FieldSpec
	Value string
}

// TextListField asks a list of entries; Options are the values the repository knows, each with its badge.
type TextListField struct {
	FieldSpec
	Entries []string
	Options []Option
}

// ConfirmField asks yes or no; Yes and No are how each answer reads, when a field words them.
type ConfirmField struct {
	FieldSpec
	Value        bool
	Yes, No      Option
	Rememberable bool
	Remember     bool
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

// Flags spells each field path as the argument or flag that sets it; the first field answering a path gives it.
func (f Form) Flags() map[string]string {
	flags := map[string]string{}
	for _, field := range f.Fields {
		spec := field.Spec()
		if _, seen := flags[spec.Path]; !seen {
			flags[spec.Path] = spec.Flag
		}
	}

	return flags
}

// Usages are the usage of each field's flag, by path: its Usage, else its Help; the first field answering a path gives it.
func (f Form) Usages() map[string]string {
	usages := map[string]string{}
	for _, field := range f.Fields {
		spec := field.Spec()
		if _, seen := usages[spec.Path]; !seen {
			usages[spec.Path] = cmp.Or(spec.Usage, spec.Help)
		}
	}

	return usages
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

// One is the single value given to a field, else the problem of a count other than one.
func One(params AnswerParams) (string, error) {
	if len(params.Values) != 1 {
		return "", Invalid([]FieldError{{Path: params.Path, Code: "one_value", Template: "{path} takes one value, not {count}", Params: Params{"count": strconv.Itoa(len(params.Values))}}})
	}

	return params.Values[0], nil
}

// Yes is the answer given to a ConfirmField, spelled as its Answer spells it: true or false.
func Yes(params AnswerParams) (bool, error) {
	value, err := One(params)
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
