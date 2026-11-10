package configset

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// The request

// The paths of config set's own fields: the answers config.toml remembers, each spelled question=value; the digest of the config.toml the request was made on.
const (
	PathRemembered = settings.PathRemembered
	PathReplaces   = "replaces"
)

// SectionRemembered is the section of the remembered answers, after the settings'.
const SectionRemembered = "remembered"

// A Request is what the caller asks config set to change: each setting given replaces config.toml's, a setting left empty keeps it; Remembered, when given, replaces the remembered answers whole (empty forgets them all); Replaces, when given, is the digest of the config.toml the request was made on (config show's), a file changed since being refused.
type Request struct {
	settings.Given
	Remembered []string `json:"remembered"`
	Replaces   string   `json:"replaces"`
}

// Answer is the request with the field at path answered by values; a value refused leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	switch path {
	case PathRemembered:
		r.Remembered = filled(values)
		return r, nil
	case PathReplaces:
		replaces, err := domain.One[string](path, values)
		if err != nil {
			return r, err
		}
		r.Replaces = replaces
		return r, nil
	}

	// a value left blank would leave the setting as config.toml holds it: as a flag's answer it is none, refused
	if slices.Contains([]string{settings.PathBasePath, settings.PathBaseBranch, settings.PathEnvStrategy}, path) && len(values) == 1 && strings.TrimSpace(values[0]) == "" {
		return r, domain.Invalid([]domain.FieldError{domain.Required(path)})
	}
	given, err := r.Given.Answer(path, values)
	if err != nil {
		return r, err
	}
	r.Given = given
	return r, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves it as config.toml holds it.
func (r Request) Values(path string) []string {
	switch path {
	case PathRemembered:
		return r.Remembered
	case PathReplaces:
		return domain.Spelled(r.Replaces)
	}

	return r.Given.Values(path)
}

// filled are the values that are not blank, an empty list rather than nil, since a list given empty is an answer.
func filled(values []string) []string {
	kept := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			kept = append(kept, value)
		}
	}

	return kept
}

// ----------------------------------------------------------------------------
// The form

// Questions are config.toml's settings as a form, each as given else as config.toml holds it, its remembered answers, and the digest of the file the request was made on.
type Questions struct {
	settings.Questions
	Remembered domain.Answer[[]string]
	Replaces   string
}

// NewQuestions settles every setting from the request and the facts: a setting given takes its answer, the others config.toml's.
func NewQuestions(facts Facts, req Request) Questions {
	questions := Questions{Questions: settings.NewQuestions(settings.QuestionsParams{Command: Name, Given: req.Given, Facts: facts.Settings}), Replaces: req.Replaces}
	switch {
	case req.Remembered != nil:
		questions.Remembered = domain.Answer[[]string]{Value: req.Remembered, Origin: domain.OriginRequest}
	case facts.Settings.Saved != nil:
		questions.Remembered = domain.Answer[[]string]{Value: Entries(facts.Settings.Saved.Config.Remembered), Origin: domain.OriginConfig}
	default:
		questions.Remembered = domain.Answer[[]string]{Value: []string{}, Origin: domain.OriginDefault}
	}

	return questions
}

// Entries are remembered answers spelled as the remembered field takes them, question=value, by question.
func Entries(remembered map[string]string) []string {
	entries := make([]string, 0, len(remembered))
	for _, question := range slices.Sorted(maps.Keys(remembered)) {
		entries = append(entries, question+"="+remembered[question])
	}

	return entries
}

// Form is the schema of the request: the settings in the order a wizard asks them, then the remembered answers, with the request's problems.
func (q Questions) Form() domain.Form {
	fields := append(q.Fields(),
		domain.TextListField{FieldSpec: domain.FieldSpec{Path: PathRemembered, Section: SectionRemembered, Origin: q.Remembered.Origin, Skip: domain.SkipFlagOnly}, Entries: q.Remembered.Value, Options: rememberable()},
		domain.TextField{FieldSpec: domain.FieldSpec{Path: PathReplaces, Skip: domain.SkipFlagOnly}, Value: q.Replaces},
	)
	problems := q.Problems()
	return domain.Form{Fields: fields, Problems: problems, Sections: domain.SectionsOf(append(settings.Sections(), SectionRemembered), fields, problems)}
}

// rememberable are every answer a question may remember, spelled as the remembered field takes them.
func rememberable() []domain.Option {
	var options []domain.Option
	for _, question := range project.RememberIDs() {
		values, _ := project.RememberableValues(question)
		for _, value := range values {
			options = append(options, domain.Option{Value: question + "=" + value})
		}
	}

	return options
}

// CodeRememberedTwice refuses two answers remembered for one question.
const CodeRememberedTwice = Name + ".remembered_twice"

// Problems are the settings' problems, the remembered answers' read from the request's rather than config.toml's: each that is no question's, holds a value its question cannot remember, or answers a question already answered.
func (q Questions) Problems() []domain.FieldError {
	problems := slices.DeleteFunc(q.Questions.Problems(), func(problem domain.FieldError) bool {
		return strings.HasPrefix(problem.Path, PathRemembered+".")
	})
	var seen []string
	for index, entry := range q.Remembered.Value {
		at := PathRemembered + "[" + strconv.Itoa(index) + "]"
		question, value, _ := strings.Cut(entry, "=")
		allowed, known := project.RememberableValues(question)
		switch {
		case !known:
			problems = append(problems, domain.FieldError{Path: at, Code: Name + "." + settings.CodeRememberedUnknown, Template: "{path} {value} remembers no answer: it is none of {accepted}", Params: domain.Params{"value": question}, Accepted: project.RememberIDs()})
		case !slices.Contains(allowed, value):
			problems = append(problems, domain.FieldError{Path: at, Code: Name + "." + settings.CodeRememberedNotAllowed, Template: "{path} {question} cannot remember {value}: it is none of {accepted}", Params: domain.Params{"value": value, "question": question}, Accepted: allowed})
		case slices.Contains(seen, question):
			problems = append(problems, domain.FieldError{Path: at, Code: CodeRememberedTwice, Template: "{path} answers {value} twice", Params: domain.Params{"value": question}})
		}
		seen = append(seen, question)
	}

	return problems
}

// remembered are the remembered answers as config.toml holds them, by question.
func (q Questions) remembered() map[string]string {
	remembered := make(map[string]string, len(q.Remembered.Value))
	for _, entry := range q.Remembered.Value {
		question, value, _ := strings.Cut(entry, "=")
		remembered[question] = value
	}

	return remembered
}
