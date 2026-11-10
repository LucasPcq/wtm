package domain

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Form is a command's request as a schema a surface renders without knowing the command: its fields in the order a wizard asks them, as data each surface words its own way, the request's problems, each naming a field, an entry of one, or a field the form does not list (remember[0]), and the sections its fields fall in, in order, none for a form of one part.
type Form struct {
	Fields   []Field
	Problems []FieldError
	Sections []Section
}

// Section is a part of a form, which its fields name by ID (FieldSpec.Section): where it stands, why when it is skipped or to check (Reason, a code, with its Params; each surface words it), and the sections it waits for before it can be answered.
type Section struct {
	ID       string       `json:"id"`
	State    SectionState `json:"state"`
	Reason   string       `json:"reason,omitempty"`
	Params   Params       `json:"params,omitempty"`
	WaitsFor []string     `json:"waits_for,omitempty"`
}

// SectionState is where a section of a form stands.
type SectionState string

// The states of a section.
const (
	SectionToDo     SectionState = "to_do"    // a field of it is not answered yet
	SectionAnswered SectionState = "answered" // every field of it is answered, with no problem
	SectionToCheck  SectionState = "to_check" // a field of it has a problem
	SectionSkipped  SectionState = "skipped"  // nothing in it is asked
)

// SectionsOf are the sections ids of fields, in order, each as its fields and problems stand: skipped when every field of it is, to do when a field of it is required and unanswered, to check when another problem names one of its fields, else answered.
func SectionsOf(ids []string, fields []Field, problems []FieldError) []Section {
	sections := make([]Section, 0, len(ids))
	for _, id := range ids {
		section := Section{ID: id, State: SectionSkipped}
		for _, field := range fields {
			spec := field.Spec()
			if spec.Section != id || spec.Skip != "" {
				continue
			}
			if section.State == SectionSkipped {
				section.State = SectionAnswered
			}
			for _, problem := range ProblemsOf(problems, spec.Path) {
				switch {
				case problem.Code == CodeRequired:
					section.State = SectionToDo
				case section.State != SectionToDo:
					section.State = SectionToCheck
				}
			}
		}
		sections = append(sections, section)
	}

	return sections
}

// MarshalJSON is the form as a surface receives it: each field with its "kind" (select, multi_select, text, text_list, confirm, hooks) first, then what that kind carries, the request's problems and the form's sections.
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
		Sections []Section         `json:"sections"`
	}{fields, orEmpty(f.Problems), orEmpty(f.Sections)})
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
	case HooksField:
		field.Entries, field.Found, field.Dirs = orEmpty(field.Entries), orEmpty(field.Found), orEmpty(field.Dirs)
		return "hooks", field
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

// PathAsk is the path of the request's --ask: true to ask again the questions whose answer is remembered.
const PathAsk = "ask"

// The paths of config.toml's settings, one per value it holds, which init asks and a change to the settings writes.
const (
	PathBasePath    = "base_path"
	PathBaseBranch  = "base_branch"
	PathEnvStrategy = "env_strategy"
	PathEnvFiles    = "env_files"
	PathOnCreate    = "on_create"
	PathOnClean     = "on_clean"
)

// The paths of the questions every command that creates a worktree asks, the fields --remember accepts: where its .env files come from, how it stands against its source, whether its source is fast-forwarded first.
const (
	PathEnvFrom     = "env_from"
	PathIsolation   = "isolation"
	PathFastForward = "fast_forward"
)

//go-sumtype:decl Field

// Field is one field of a Form, of one kind: SelectField, MultiSelectField, TextField, TextListField, ConfirmField or HooksField, each carrying only what is valid for it.
type Field interface {
	// Spec is what a field of every kind has.
	Spec() FieldSpec
	// Answer is the field's answer as the values its command's Request.Answer takes.
	Answer() []string
	field()
}

// FieldSpec is what a field of every kind has; its words (its name, what its answer decides, its flag on the command line) are each surface's own, by its path.
type FieldSpec struct {
	Path    string `json:"path"`              // the request field it answers, as a field error names it
	Origin  Origin `json:"origin,omitempty"`  // where its answer came from, empty when the field does not say
	Skip    Skip   `json:"skip,omitempty"`    // why it is not asked, empty when it is
	Section string `json:"section,omitempty"` // the ID of the form's section it falls in, empty for a form of one part
}

// Spec is the spec itself, so each kind of field has it by embedding.
func (s FieldSpec) Spec() FieldSpec {
	return s
}

// SelectField asks one of its options; Pinned is the option listed first, Remembered the answer remembered for it, Remember whether this run's answer is to be, Refreshable whether its options are facts the user may read again while it is asked, Closed whether its options are every value it accepts, the same in every repository, so a surface may word each one its own way; Refused are the values it leaves out of its options, each as the problem it would raise if given.
type SelectField struct {
	FieldSpec
	Value        string       `json:"value"`
	Options      []Option     `json:"options"`
	Pinned       string       `json:"pinned,omitempty"`
	Remembered   string       `json:"remembered,omitempty"`
	Rememberable bool         `json:"rememberable"`
	Remember     bool         `json:"remember"`
	Refreshable  bool         `json:"refreshable"`
	Closed       bool         `json:"closed"`
	Refused      []FieldError `json:"refused,omitempty"`
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

// TextListField asks a list of entries; Options are the values the repository knows, each with what it knows of it.
type TextListField struct {
	FieldSpec
	Entries []string `json:"entries"`
	Options []Option `json:"options"`
}

// ConfirmField asks yes or no, answered true or false, no listed first when NoFirst; Remembered is the answer remembered for it, nil when none.
type ConfirmField struct {
	FieldSpec
	Value        bool  `json:"value"`
	NoFirst      bool  `json:"no_first"`
	Remembered   *bool `json:"remembered,omitempty"`
	Rememberable bool  `json:"rememberable"`
	Remember     bool  `json:"remember"`
}

// HooksField asks a list of hooks, in the order they run: each a command, the folder it runs in and whether its failure lets the next ones run. Found are the hooks the repository suggests, each with the file it was found in, and Dirs the folders of the repository a hook can run in.
type HooksField struct {
	FieldSpec
	Entries []project.Hook      `json:"entries"`
	Found   []project.FoundHook `json:"found"`
	Dirs    []string            `json:"dirs"`
}

// Answer is the option selected.
func (f SelectField) Answer() []string { return []string{f.Value} }

// Answer is the options picked.
func (f MultiSelectField) Answer() []string { return f.Picked }

// Answer is the text typed.
func (f TextField) Answer() []string { return []string{f.Value} }

// Answer is the entries.
func (f TextListField) Answer() []string { return f.Entries }

// Answer is each hook spelled as HookOf reads it.
func (f HooksField) Answer() []string {
	spelled := make([]string, 0, len(f.Entries))
	for _, hook := range f.Entries {
		spelled = append(spelled, SpellHook(hook))
	}

	return spelled
}

// Answer is true or false, as strconv spells it.
func (f ConfirmField) Answer() []string { return []string{strconv.FormatBool(f.Value)} }

func (SelectField) field()      {}
func (MultiSelectField) field() {}
func (TextField) field()        {}
func (TextListField) field()    {}
func (ConfirmField) field()     {}
func (HooksField) field()       {}

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
	case MultiSelectField, TextField, TextListField, HooksField:
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

// CodeOneValue is the field-error code of a field given a count of values other than one.
const CodeOneValue = "one_value"

// One is the single value given to the field at path, else the problem of a count other than one.
func One[T ~string](path string, values []string) (T, error) {
	if len(values) != 1 {
		return "", Invalid([]FieldError{{Path: path, Code: CodeOneValue, Template: "{path} takes one value, not {count}", Params: Params{"count": strconv.Itoa(len(values))}}})
	}

	return T(values[0]), nil
}

// Yes is the answer given to a ConfirmField, spelled as its Answer and its options spell it: true or false.
func Yes(path string, values []string) (bool, error) {
	value, err := One[string](path, values)
	if err != nil {
		return false, err
	}

	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	return false, Invalid([]FieldError{{Path: path, Code: CodeOneOf, Template: "{path} must be one of {accepted}, not {value}", Params: Params{"value": value}, Accepted: []string{"true", "false"}}})
}

// Answerable is a command's request, which takes the answer of a field at a time.
type Answerable[R any] interface {
	Answer(path string, values []string) (R, error)
}

// Answered is the answer to the field at Path, as its command's Request.Answer takes it.
type Answered struct {
	Path   string   `json:"path"`
	Values []string `json:"values"`
}

// MarshalJSON writes the answer with its values always an array, none included, as a surface types them.
func (a Answered) MarshalJSON() ([]byte, error) {
	type plain Answered
	a.Values = orEmpty(a.Values)
	return encoded(plain(a))
}

// Answers are the answer of each field of the form, in order, then, when a field is rememberable, the fields whose answer is to be remembered: the inverse of Fold, which a surface or a test folds back into the request the form stands for.
func (f Form) Answers() []Answered {
	answers := make([]Answered, 0, len(f.Fields)+1)
	for _, field := range f.Fields {
		answers = append(answers, Answered{Path: field.Spec().Path, Values: field.Answer()})
	}
	if len(f.Rememberable()) == 0 {
		return answers
	}

	return append(answers, Answered{Path: PathRemember, Values: f.Remembering()})
}

// Valued is a command's request, which says its answer to the field at path, nil when it leaves the field to its default.
type Valued interface {
	Values(path string) []string
}

// AnswersOf are the answers req gives the fields of form, a path shared by two fields once, then the fields it asks to remember and whether it asks again: what Fold folds back into req.
func AnswersOf(form Form, req Valued) []Answered {
	paths := make([]string, 0, len(form.Fields)+2)
	for _, field := range form.Fields {
		paths = append(paths, field.Spec().Path)
	}

	var answers []Answered
	seen := map[string]bool{}
	for _, path := range append(paths, PathRemember, PathAsk) {
		values := req.Values(path)
		if seen[path] || values == nil {
			continue
		}
		seen[path] = true
		answers = append(answers, Answered{Path: path, Values: values})
	}

	return answers
}

// Fold is req with each of answers given in turn; it stops at the first one req refuses.
func Fold[R Answerable[R]](req R, answers []Answered) (R, error) {
	for _, answer := range answers {
		var err error
		if req, err = req.Answer(answer.Path, answer.Values); err != nil {
			return req, err
		}
	}

	return req, nil
}

// Spelled is value as a field's answer, the way a Request's Values spells it: none when empty.
func Spelled[T ~string](value T) []string {
	if value == "" {
		return nil
	}

	return []string{string(value)}
}

// Said is the answer to a ConfirmField the request gives, spelled as Yes reads it: none when it does not say.
func Said(value *bool) []string {
	if value == nil {
		return nil
	}

	return []string{strconv.FormatBool(*value)}
}

// SaidYes is the answer of a ConfirmField whose no is its default: true when on, else none.
func SaidYes(on bool) []string {
	if !on {
		return nil
	}

	return []string{"true"}
}
