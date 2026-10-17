package form

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// A sheet is what the step of a field kind reads and answers of its [Session], whatever its command.
type sheet interface {
	Field(at int) domain.Field
	Standing(at int) Standing
	Label(path string) string
	Answer(at int, answers ...domain.Answered)
	Forget(at int)
	remembering(at int, path string, on bool, answers ...domain.Answered)
	values(path string) []string
	problems(answers ...domain.Answered) ([]domain.FieldError, error)
}

func (s *Session[U, R, Q, O]) values(path string) []string {
	return s.req.Values(path)
}

// problems are the problems of the request as answered so far with answers given too.
func (s *Session[U, R, Q, O]) problems(answers ...domain.Answered) ([]domain.FieldError, error) {
	questions, err := s.Check(answers...)
	if err != nil {
		return nil, err
	}

	return questions.Form().Problems, nil
}

// fieldStep returns the step that asks field, at its place in the form, by its kind.
func fieldStep(s sheet, at int, field domain.Field) Step {
	base := asked{sheet: s, at: at}
	switch field := field.(type) {
	case domain.SelectField:
		if field.Refreshable {
			return &refreshedSelectStep{selectStep{asked: base}}
		}
		return &selectStep{asked: base}
	case domain.ConfirmField:
		return &confirmStep{asked: base}
	case domain.TextListField:
		return &textListStep{asked: base}
	case domain.TextField:
		return &textStep{asked: base}
	case domain.MultiSelectField:
		return &multiSelectStep{asked: base}
	}

	return nil
}

// asked is a step's place in its session, which gives every step of a field kind its label, description, standing and reset.
type asked struct {
	sheet sheet
	at    int
}

// Label is the field's label.
func (a asked) Label() string {
	return a.sheet.Field(a.at).Spec().Label
}

// Description is the field's help.
func (a asked) Description() string {
	return a.sheet.Field(a.at).Spec().Help
}

// Standing is where the field's answer stands.
func (a asked) Standing() Standing {
	return a.sheet.Standing(a.at)
}

// Reset forgets the step's answer.
func (a asked) Reset() {
	a.sheet.Forget(a.at)
}

func (a asked) path() string {
	return a.sheet.Field(a.at).Spec().Path
}

// trailOf is the answer of field as the trail shows it.
func trailOf(field domain.Field) string {
	switch field := field.(type) {
	case domain.SelectField:
		return shortOf(field.Options, field.Value)
	case domain.ConfirmField:
		return confirmed(field, field.Value)
	case domain.TextListField:
		return strings.Join(field.Entries, ", ")
	case domain.TextField:
		return field.Value
	case domain.MultiSelectField:
		return strings.Join(field.Picked, ", ")
	}

	return ""
}

// shortOf names value as the trail does: the Short of its option, else the value itself.
func shortOf(options []domain.Option, value string) string {
	at := slices.IndexFunc(options, func(o domain.Option) bool { return o.Value == value })
	if at < 0 {
		return value
	}

	return cmp.Or(options[at].Short, value)
}

// ----------------------------------------------------------------------------
// A select

// selectStep asks one of a field's options in a select list, the pinned one first, with the "always use this answer" toggle when the answer can be remembered.
type selectStep struct {
	asked
	list *SelectList
}

// Open builds the list on the answer so far; a remembered answer starts the list on it, the toggle ticked.
func (s *selectStep) Open() tea.Cmd {
	s.list = s.build()
	return nil
}

func (s *selectStep) build() *SelectList {
	field, _ := s.sheet.Field(s.at).(domain.SelectField)
	options := Options(field.Options)
	if at := slices.IndexFunc(options, func(o Option) bool { return o.Value == field.Pinned }); field.Pinned != "" && at > 0 {
		pinned := options[at]
		options = append([]Option{pinned}, slices.Delete(options, at, at+1)...)
	}

	params := SelectParams{Options: options, Start: cmp.Or(field.Value, field.Pinned), Toggle: field.Rememberable, ToggleOn: field.Remember}
	if field.Remembered != "" {
		params.Start, params.ToggleOn = field.Remembered, true
	}

	return NewSelectList(params)
}

// Field is the select list.
func (s *selectStep) Field() Field {
	return s.list
}

// Answer records the option selected, and whether to remember it.
func (s *selectStep) Answer() string {
	field, _ := s.sheet.Field(s.at).(domain.SelectField)
	answered := domain.Answered{Path: field.Path, Values: []string{s.list.Value()}}
	if field.Rememberable {
		s.sheet.remembering(s.at, field.Path, s.list.Remember(), answered)
	} else {
		s.sheet.Answer(s.at, answered)
	}

	return shortOf(field.Options, s.list.Value())
}

// refreshedSelectStep is a select whose options are facts the user may read again with r.
type refreshedSelectStep struct {
	selectStep
}

// Refreshable reports whether the list can be rebuilt now: not while the user types a filter.
func (s *refreshedSelectStep) Refreshable() bool {
	return !s.list.Filtering()
}

// Refreshed rebuilds the list from the options read again, keeping the option selected.
func (s *refreshedSelectStep) Refreshed() {
	kept := s.list.Value()
	s.list = s.build()
	s.list.StartOn(kept)
}

// Options are the rows of a select list that offer options, each read as its label, else its value.
func Options(found []domain.Option) []Option {
	options := make([]Option, 0, len(found))
	for _, o := range found {
		options = append(options, Option{Label: cmp.Or(o.Label, o.Value), Value: o.Value, Badge: o.Badge})
	}

	return options
}

// ----------------------------------------------------------------------------
// A confirmation

const (
	yesLabel = "Yes"
	noLabel  = "No"
)

// confirmStep asks yes or no in a select list of the field's two options; one whose answer can be remembered sets them apart by a rule, above the "always use this answer" toggle.
type confirmStep struct {
	asked
	list *SelectList
}

// Open builds the list on the answer so far; a remembered answer ticks the toggle.
func (s *confirmStep) Open() tea.Cmd {
	field, _ := s.sheet.Field(s.at).(domain.ConfirmField)
	yes := Option{Label: cmp.Or(field.Yes.Label, yesLabel), Value: strconv.FormatBool(true)}
	no := Option{Label: cmp.Or(field.No.Label, noLabel), Value: strconv.FormatBool(false)}

	options := []Option{yes, no}
	if field.NoFirst {
		options = []Option{no, yes}
	}
	if field.Rememberable {
		options = slices.Insert(options, 1, Option{Separator: true})
	}

	ticked := field.Remember || field.Origin == domain.OriginRemembered
	s.list = NewSelectList(SelectParams{Options: options, Start: strconv.FormatBool(field.Value), Toggle: field.Rememberable, ToggleOn: ticked})
	return nil
}

// Field is the select list.
func (s *confirmStep) Field() Field {
	return s.list
}

// Answer records yes or no, and whether to remember it.
func (s *confirmStep) Answer() string {
	field, _ := s.sheet.Field(s.at).(domain.ConfirmField)
	answered := domain.Answered{Path: field.Path, Values: []string{s.list.Value()}}
	if field.Rememberable {
		s.sheet.remembering(s.at, field.Path, s.list.Remember(), answered)
	} else {
		s.sheet.Answer(s.at, answered)
	}

	return confirmed(field, s.list.Value() == strconv.FormatBool(true))
}

// confirmed names the answer yes of field as the trail does: the Short of its option, else yes or no.
func confirmed(field domain.ConfirmField, yes bool) string {
	if yes {
		return cmp.Or(field.Yes.Short, "yes")
	}

	return cmp.Or(field.No.Short, "no")
}

// ----------------------------------------------------------------------------
// A text list

// textListStep asks a list of entries, each checked against the repository as it is added, each badged with what the repository knows of it.
type textListStep struct {
	asked
	list *TextList
}

// Open builds the list from the entries answered so far and starts the cursor blinking.
func (s *textListStep) Open() tea.Cmd {
	s.list = NewTextList(s.sheet.values(s.path()), s.check, s.badge)
	return textinput.Blink
}

// check is the first problem of the list itself or of an entry from check.From on, as the step shows it.
func (s *textListStep) check(check EntryCheck) error {
	path := s.path()
	problems, err := s.sheet.problems(domain.Answered{Path: path, Values: check.Entries})
	if err != nil {
		return Sentence(err)
	}

	for _, found := range problems {
		if found.Path == path {
			return Problem(found, s.sheet.Label(found.Path))
		}

		index, entry := strings.CutPrefix(found.Path, path+"[")
		at, err := strconv.Atoi(strings.TrimSuffix(index, "]"))
		if entry && err == nil && at >= check.From {
			return Problem(found, s.sheet.Label(found.Path))
		}
	}

	return nil
}

// badge is the badge of the option an entry names, "" for a value the field does not offer.
func (s *textListStep) badge(entry string) string {
	field, _ := s.sheet.Field(s.at).(domain.TextListField)
	at := slices.IndexFunc(field.Options, func(o domain.Option) bool { return o.Value == entry })
	if at < 0 {
		return ""
	}

	return field.Options[at].Badge
}

// Field is the text list.
func (s *textListStep) Field() Field {
	return s.list
}

// Answer records the entries.
func (s *textListStep) Answer() string {
	s.sheet.Answer(s.at, domain.Answered{Path: s.path(), Values: s.list.Entries()})
	return strings.Join(s.list.Entries(), ", ")
}

// ----------------------------------------------------------------------------
// A text

// textStep asks one line of text, checked against the repository once typed.
type textStep struct {
	asked
	text *Text
}

// Open builds the text field on the answer so far and starts the cursor blinking.
func (s *textStep) Open() tea.Cmd {
	s.text = NewText(strings.Join(s.sheet.values(s.path()), ""), s.check)
	return textinput.Blink
}

// check is the first problem of the value typed, as the step shows it.
func (s *textStep) check(value string) error {
	problems, err := s.sheet.problems(domain.Answered{Path: s.path(), Values: []string{value}})
	if err != nil {
		return Sentence(err)
	}

	if found := domain.ProblemsOf(problems, s.path()); len(found) > 0 {
		return Problem(found[0], s.sheet.Label(found[0].Path))
	}

	return nil
}

// Field is the text field.
func (s *textStep) Field() Field {
	return s.text
}

// Answer records the value typed.
func (s *textStep) Answer() string {
	s.sheet.Answer(s.at, domain.Answered{Path: s.path(), Values: []string{s.text.Value()}})
	return s.text.Value()
}

// ----------------------------------------------------------------------------
// A multi-select

// multiSelectStep asks any of a field's options, those answered so far picked; none picked is refused with the field's Empty problem.
type multiSelectStep struct {
	asked
	picks *MultiSelect
}

// Open builds the multi-select.
func (s *multiSelectStep) Open() tea.Cmd {
	field, _ := s.sheet.Field(s.at).(domain.MultiSelectField)
	picked := s.sheet.values(field.Path)
	options := make([]Pick, 0, len(field.Options))
	for _, o := range field.Options {
		options = append(options, Pick{Label: cmp.Or(o.Label, o.Value), Value: o.Value, Tag: o.Badge, Picked: slices.Contains(picked, o.Value)})
	}

	s.picks = NewMultiSelect(MultiSelectParams{Options: options, Empty: field.Empty})
	return nil
}

// Field is the multi-select.
func (s *multiSelectStep) Field() Field {
	return s.picks
}

// Answer records the options picked.
func (s *multiSelectStep) Answer() string {
	s.sheet.Answer(s.at, domain.Answered{Path: s.path(), Values: s.picks.Values()})
	return strings.Join(s.picks.Values(), ", ")
}
