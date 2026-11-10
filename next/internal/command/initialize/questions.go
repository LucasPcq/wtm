package initialize

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of init: the settings it gives, each left to config.toml, what the repository suggests or its default when not given.
type Request struct {
	settings.Given
}

// Answer is the request with the setting at path answered by values. A value the setting cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	given, err := r.Given.Answer(path, values)
	if err != nil {
		return r, err
	}

	r.Given = given
	return r, nil
}

// Values is the request's answer to the setting at path, spelled as Answer takes it; nil when the request leaves it to its default.
func (r Request) Values(path string) []string {
	return r.Given.Values(path)
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the settings, each settled on the facts, and their problems.
type Questions struct {
	settings.Questions
}

// NewQuestions builds the settings for the request on the facts: a first init takes what the repository suggests, a re-init what config.toml holds.
func NewQuestions(f Facts, req Request) Questions {
	return Questions{settings.NewQuestions(settings.QuestionsParams{Command: Name, Given: req.Given, Facts: f.Settings})}
}

// Form is the questions as the schema a surface renders: the worktrees' folder and base branch, where .env files come from and which, then the hooks. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	return domain.Form{Fields: q.Fields(), Problems: q.Problems()}
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, every setting answered; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if problems := questions.Problems(); len(problems) > 0 {
		return Request{}, domain.Invalid(problems)
	}

	return Request{Given: questions.Given()}, nil
}
