package form

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// A sheet is what the step of a field kind reads and answers of its [Session], whatever its command.
type sheet interface {
	Field(at int) domain.Field
	Words(at int) Words
	Standing(at int) Standing
	Label(path string) string
	Problem(found domain.FieldError, label string) error
	Answer(at int, answers ...domain.Answered)
	Forget(at int)
	remembering(at int, path string, on bool, answers ...domain.Answered)
	values(path string) []string
	problems(answers ...domain.Answered) ([]domain.FieldError, error)
}

func (s *Session[R, Q, O]) values(path string) []string {
	return s.req.Values(path)
}

// problems are the problems of the request as answered so far with answers given too.
func (s *Session[R, Q, O]) problems(answers ...domain.Answered) ([]domain.FieldError, error) {
	questions, err := s.Check(answers...)
	if err != nil {
		return nil, err
	}

	return questions.Form().Problems, nil
}

// fieldStep returns the step that asks field, at its place in the form, by its kind.
func fieldStep(s sheet, at int, field domain.Field) Step {
	base := Asked{sheet: s, at: at}
	switch field := field.(type) {
	case domain.SelectField:
		if field.Refreshable {
			return &refreshedSelectStep{selectStep{Asked: base}}
		}
		return &selectStep{Asked: base}
	case domain.ConfirmField:
		return &confirmStep{Asked: base}
	case domain.TextListField:
		return &textListStep{Asked: base}
	case domain.TextField:
		return &TextStep{Asked: base}
	case domain.MultiSelectField:
		return &MultiSelectStep{Asked: base}
	}

	return nil
}

// Asked is a step's place in its session, which gives every step of a field kind, and a command's own steps that embed it, its label, description, standing and reset.
type Asked struct {
	sheet sheet
	at    int
}

// Label is the field's label.
func (a Asked) Label() string {
	return a.sheet.Words(a.at).Label
}

// Description is the field's help.
func (a Asked) Description() string {
	return a.sheet.Words(a.at).Help
}

// Standing is where the field's answer stands.
func (a Asked) Standing() Standing {
	return a.sheet.Standing(a.at)
}

// Reset forgets the step's answer.
func (a Asked) Reset() {
	a.sheet.Forget(a.at)
}

func (a Asked) path() string {
	return a.sheet.Field(a.at).Spec().Path
}

// trailOf is the answer of field as the trail shows it, in its words.
func trailOf(field domain.Field, words Words) string {
	switch field := field.(type) {
	case domain.SelectField:
		return shortOf(words, field.Value)
	case domain.ConfirmField:
		return confirmed(words, field.Value)
	case domain.TextListField:
		return strings.Join(field.Entries, ", ")
	case domain.TextField:
		return field.Value
	case domain.MultiSelectField:
		return strings.Join(field.Picked, ", ")
	}

	return ""
}

// ----------------------------------------------------------------------------
// A select

// selectStep asks one of a field's options in a select list, the pinned one first, with the "always use this answer" toggle when the answer can be remembered.
type selectStep struct {
	Asked
	list *widget.SelectList
}

// Open builds the list on the answer so far; a remembered answer starts the list on it, the toggle ticked.
func (s *selectStep) Open() tea.Cmd {
	s.list = s.build()
	return nil
}

func (s *selectStep) build() *widget.SelectList {
	field, _ := s.sheet.Field(s.at).(domain.SelectField)
	options := Options(field.Options, s.sheet.Words(s.at))
	if at := slices.IndexFunc(options, func(o widget.Option) bool { return o.Value == field.Pinned }); field.Pinned != "" && at > 0 {
		pinned := options[at]
		options = append([]widget.Option{pinned}, slices.Delete(options, at, at+1)...)
	}

	params := widget.SelectParams{Options: options, Start: cmp.Or(field.Value, field.Pinned), Toggle: field.Rememberable, ToggleOn: field.Remember}
	if field.Remembered != "" {
		params.Start, params.ToggleOn = field.Remembered, true
	}

	return widget.NewSelectList(params)
}

// Widget is the select list.
func (s *selectStep) Widget() widget.Widget {
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

	return shortOf(s.sheet.Words(s.at), s.list.Value())
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

// ----------------------------------------------------------------------------
// A confirmation

const (
	yesLabel = "Yes"
	noLabel  = "No"
)

// confirmStep asks yes or no in a select list of the field's two options; one whose answer can be remembered sets them apart by a rule, above the "always use this answer" toggle.
type confirmStep struct {
	Asked
	list *widget.SelectList
}

// Open builds the list on the answer so far; a remembered answer starts the list on it, the toggle ticked.
func (s *confirmStep) Open() tea.Cmd {
	field, _ := s.sheet.Field(s.at).(domain.ConfirmField)
	words := s.sheet.Words(s.at).Options
	yes := widget.Option{Label: cmp.Or(words["true"].Label, yesLabel), Value: strconv.FormatBool(true)}
	no := widget.Option{Label: cmp.Or(words["false"].Label, noLabel), Value: strconv.FormatBool(false)}

	options := []widget.Option{yes, no}
	if field.NoFirst {
		options = []widget.Option{no, yes}
	}
	if field.Rememberable {
		options = slices.Insert(options, 1, widget.Option{Separator: true})
	}

	params := widget.SelectParams{Options: options, Start: strconv.FormatBool(field.Value), Toggle: field.Rememberable, ToggleOn: field.Remember}
	if field.Remembered != nil {
		params.Start, params.ToggleOn = strconv.FormatBool(*field.Remembered), true
	}

	s.list = widget.NewSelectList(params)
	return nil
}

// Widget is the select list.
func (s *confirmStep) Widget() widget.Widget {
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

	return confirmed(s.sheet.Words(s.at), s.list.Value() == strconv.FormatBool(true))
}

// confirmed names the answer yes as the trail does: the Short its words give it, else yes or no.
func confirmed(words Words, yes bool) string {
	if yes {
		return cmp.Or(words.Options["true"].Short, "yes")
	}

	return cmp.Or(words.Options["false"].Short, "no")
}

// ----------------------------------------------------------------------------
// A text list

const newBadge = "new"

// textListStep asks a list of entries, each checked against the repository as it is added, each badged with what the repository knows of it.
type textListStep struct {
	Asked
	list *widget.TextList
}

// Open builds the list from the entries answered so far and starts the cursor blinking.
func (s *textListStep) Open() tea.Cmd {
	s.list = widget.NewTextList(s.sheet.values(s.path()), s.check, s.badge)
	return textinput.Blink
}

// check is the first problem of the list itself or of an entry from check.From on, as the step shows it.
func (s *textListStep) check(check widget.EntryCheck) error {
	path := s.path()
	problems, err := s.sheet.problems(domain.Answered{Path: path, Values: check.Entries})
	if err != nil {
		return Sentence(err)
	}

	for _, found := range problems {
		if found.Path == path {
			return s.sheet.Problem(found, s.sheet.Label(found.Path))
		}

		index, entry := strings.CutPrefix(found.Path, path+"[")
		at, err := strconv.Atoi(strings.TrimSuffix(index, "]"))
		if entry && err == nil && at >= check.From {
			return s.sheet.Problem(found, s.sheet.Label(found.Path))
		}
	}

	return nil
}

// badge is the badge of the option an entry names, "new" for a value the field does not offer.
func (s *textListStep) badge(entry string) string {
	field, _ := s.sheet.Field(s.at).(domain.TextListField)
	at := slices.IndexFunc(field.Options, func(o domain.Option) bool { return o.Value == entry })
	if at < 0 {
		return newBadge
	}

	return badgeOf(field.Options[at])
}

// Widget is the text list.
func (s *textListStep) Widget() widget.Widget {
	return s.list
}

// Answer records the entries.
func (s *textListStep) Answer() string {
	s.sheet.Answer(s.at, domain.Answered{Path: s.path(), Values: s.list.Entries()})
	return strings.Join(s.list.Entries(), ", ")
}

// ----------------------------------------------------------------------------
// A text

// TextStep asks one line of text, checked against the repository once typed: by Check when a command's own step sets it, else by the problems the repository finds in it.
type TextStep struct {
	Asked
	Check func(value string) error
	text  *widget.Text
}

// Open builds the text field on the answer so far and starts the cursor blinking.
func (s *TextStep) Open() tea.Cmd {
	check := s.check
	if s.Check != nil {
		check = s.Check
	}

	s.text = widget.NewText(strings.Join(s.sheet.values(s.path()), ""), check)
	return textinput.Blink
}

// check is the first problem of the value typed, as the step shows it.
func (s *TextStep) check(value string) error {
	problems, err := s.sheet.problems(domain.Answered{Path: s.path(), Values: []string{value}})
	if err != nil {
		return Sentence(err)
	}

	if found := domain.ProblemsOf(problems, s.path()); len(found) > 0 {
		return s.sheet.Problem(found[0], s.sheet.Label(found[0].Path))
	}

	return nil
}

// Widget is the text field.
func (s *TextStep) Widget() widget.Widget {
	return s.text
}

// Answer records the value typed.
func (s *TextStep) Answer() string {
	s.sheet.Answer(s.at, domain.Answered{Path: s.path(), Values: []string{s.text.Value()}})
	return s.text.Value()
}

// ----------------------------------------------------------------------------
// A multi-select

// MultiSelectStep asks any of a field's options, those answered so far picked, else those the field picks by default, each tag in the Tone a command's own step gives it (muted otherwise); none picked is refused with the Empty problem of its words.
type MultiSelectStep struct {
	Asked
	Tone  func(option domain.Option) widget.Tone
	picks *widget.MultiSelect
}

// Open builds the multi-select.
func (s *MultiSelectStep) Open() tea.Cmd {
	field, _ := s.sheet.Field(s.at).(domain.MultiSelectField)
	words := s.sheet.Words(s.at)
	picked := s.sheet.values(field.Path)
	if len(picked) == 0 {
		picked = field.Picked
	}
	options := make([]widget.Pick, 0, len(field.Options))
	for _, o := range field.Options {
		pick := widget.Pick{Label: optionLabel(o, words), Value: o.Value, Tag: badgeOf(o), Picked: slices.Contains(picked, o.Value)}
		if s.Tone != nil {
			pick.Tone = s.Tone(o)
		}
		options = append(options, pick)
	}

	s.picks = widget.NewMultiSelect(options, words.Empty)
	return nil
}

// Widget is the multi-select.
func (s *MultiSelectStep) Widget() widget.Widget {
	return s.picks
}

// Answer records the options picked.
func (s *MultiSelectStep) Answer() string {
	s.sheet.Answer(s.at, domain.Answered{Path: s.path(), Values: s.picks.Values()})
	return strings.Join(s.picks.Values(), ", ")
}
