package provision

import (
	"cmp"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// ----------------------------------------------------------------------------
// The request

// Given is what a request says to the questions of a new worktree, as given: a field left empty takes its remembered answer, its configured value or its default. A command that creates a worktree embeds it in its request.
type Given struct {
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation"`
	FastForward *bool               `json:"fast_forward"` // nil when not given, so a remembered answer can apply
}

// Answer is the request with the field at path answered by values, spelled as the field's Answer spells them; a value the field cannot take, or a path that is not one of these questions, leaves the request as it was.
func (g Given) Answer(path string, values []string) (Given, error) {
	answer := domain.AnswerParams{Path: path, Values: values}
	next := g
	var err error
	switch path {
	case PathEnvFrom:
		next.EnvFrom, err = domain.One[project.EnvStrategy](answer)
	case PathIsolation:
		next.Isolation, err = domain.One[project.Isolation](answer)
	case PathFastForward:
		var yes bool
		yes, err = domain.Yes(answer)
		next.FastForward = &yes
	default:
		err = domain.Unknown(path)
	}

	if err != nil {
		return g, err
	}

	return next, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves the field to its default, or path is not one of these questions.
func (g Given) Values(path string) []string {
	switch path {
	case PathEnvFrom:
		return domain.Spelled(g.EnvFrom)
	case PathIsolation:
		return domain.Spelled(g.Isolation)
	case PathFastForward:
		return domain.Said(g.FastForward)
	}

	return nil
}

// The paths of the questions of a new worktree.
const (
	PathEnvFrom     = "env_from"
	PathIsolation   = "isolation"
	PathFastForward = "fast_forward"
)

// ----------------------------------------------------------------------------
// The questions

// Questions are the questions every command that creates a worktree asks: where its .env files come from, how it stands against its source, and whether its source is fast-forwarded first; with the fields whose answer is to be remembered.
type Questions struct {
	EnvFrom     domain.Choice[project.EnvStrategy]
	Isolation   domain.Choice[project.Isolation]
	FastForward domain.Confirm
	Remember    []string
}

// QuestionsParams is what the questions of a new worktree settle from: the request's answers, the answers remembered in config.toml (which Ask sets aside for the run, the form still showing them), the configured env strategy, run.toml, why nothing is asked (Exists: the worktree is not created) and why the fast-forward is not (NotBehind: nothing it starts from is behind origin, worded by the command).
type QuestionsParams struct {
	Given       Given
	Remembered  map[string]string
	Ask         bool
	EnvStrategy project.EnvStrategy
	Run         project.Run
	Exists      string
	NotBehind   string
	Remember    []string
}

// NewQuestions builds the questions of a new worktree: a field given keeps its value, an empty one takes its remembered answer, its configured value or its default; none is answered by default when the worktree is not created.
func NewQuestions(params QuestionsParams) Questions {
	recalled := params.Remembered
	if params.Ask {
		params.Remembered = nil
	}

	questions := Questions{
		EnvFrom:     envFrom(params),
		Isolation:   isolation(params),
		FastForward: fastForward(params),
		Remember:    params.Remember,
	}
	questions.EnvFrom.Remembered = project.RememberedEnvStrategy(recalled[project.RememberEnvStrategy], params.EnvStrategy)
	questions.Isolation.Remembered = project.Isolation(recalled[project.RememberIsolation])
	questions.FastForward.Remembered = domain.RememberedFastForward(recalled[project.RememberSourceUpdate])
	return questions
}

func envFrom(params QuestionsParams) domain.Choice[project.EnvStrategy] {
	question := domain.Choice[project.EnvStrategy]{Options: project.EnvStrategies(), Skip: params.Exists}
	remembered := project.RememberedEnvStrategy(params.Remembered[project.RememberEnvStrategy], params.EnvStrategy)
	switch {
	case params.Given.EnvFrom != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: params.Given.EnvFrom, Origin: domain.OriginRequest}
	case params.Exists != "":
	case remembered != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: remembered, Origin: domain.OriginRemembered}
	case params.EnvStrategy != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: params.EnvStrategy, Origin: domain.OriginConfig}
	}

	return question
}

func isolation(params QuestionsParams) domain.Choice[project.Isolation] {
	question := domain.Choice[project.Isolation]{Options: project.Isolations()}
	switch {
	case params.Exists != "":
		question.Skip = params.Exists
	case !params.Run.IsolationApplies():
		question.Skip = "run.toml declares nothing a worktree isolates"
	}

	switch {
	case params.Given.Isolation != "":
		question.Answer = domain.Answer[project.Isolation]{Value: params.Given.Isolation, Origin: domain.OriginRequest}
	case params.Exists == "":
		question.Answer = domain.IsolationAnswer(project.Isolation(params.Remembered[project.RememberIsolation]), params.Run)
	}

	return question
}

func fastForward(params QuestionsParams) domain.Confirm {
	return domain.FastForwardAnswer(domain.FastForwardParams{
		Given:      params.Given.FastForward,
		Remembered: params.Remembered[project.RememberSourceUpdate],
		Skip:       cmp.Or(params.Exists, params.NotBehind),
	})
}

// Wording is how a command words the source update for its own subjects: the question's help and its two options, the engine's own when zero.
type Wording struct {
	FastForwardHelp string
	Yes, No         domain.Option
}

// Fields are the questions as the fields of a form, in the order a wizard asks them, for a command to embed in its own.
func (q Questions) Fields(wording Wording) []domain.Field {
	return []domain.Field{
		domain.SelectField{
			FieldSpec:    domain.FieldSpec{Path: PathEnvFrom, Flag: "--env-from", Label: "Env strategy", Help: "How the .env files of the new worktrees are provisioned: example (from its template), main (copied from the main checkout) or parent (from the source's worktree); defaults to config.toml's strategy", Origin: q.EnvFrom.Origin, Skip: q.EnvFrom.Skip},
			Value:        string(q.EnvFrom.Value),
			Options:      domain.Options(project.EnvStrategies()),
			Remembered:   string(q.EnvFrom.Remembered),
			Rememberable: true,
			Remember:     q.Remembers(PathEnvFrom),
		},
		domain.SelectField{
			FieldSpec:    domain.FieldSpec{Path: PathIsolation, Flag: "--isolation", Label: "Isolation", Help: domain.IsolationHelp, Origin: q.Isolation.Origin, Skip: q.Isolation.Skip},
			Value:        string(q.Isolation.Value),
			Options:      domain.Options(project.Isolations()),
			Remembered:   string(q.Isolation.Remembered),
			Rememberable: true,
			Remember:     q.Remembers(PathIsolation),
		},
		domain.ConfirmField{
			FieldSpec:    domain.FieldSpec{Path: PathFastForward, Flag: "--ff", Label: "Source update", Help: wording.FastForwardHelp, Origin: q.FastForward.Origin, Skip: q.FastForward.Skip},
			Value:        q.FastForward.Value,
			Yes:          wording.Yes,
			No:           wording.No,
			Remembered:   q.FastForward.Remembered,
			Rememberable: true,
			Remember:     q.Remembers(PathFastForward),
		},
	}
}

// Remembers reports whether the answer to the field at path is to be remembered: asked to be, and its question not skipped.
func (q Questions) Remembers(path string) bool {
	return slices.Contains(q.Remember, path) && !q.skipped(path)
}

func (q Questions) skipped(path string) bool {
	switch path {
	case PathEnvFrom:
		return q.EnvFrom.Skip != ""
	case PathIsolation:
		return q.Isolation.Skip != ""
	case PathFastForward:
		return q.FastForward.Skip != ""
	}

	return false
}

// Problems are the problems of the answers as field errors: a strategy or, given, an isolation that is not one of those accepted.
func (q Questions) Problems() []domain.FieldError {
	problems := domain.OneOf(domain.OneOfParams{Path: PathEnvFrom, Value: string(q.EnvFrom.Value), Accepted: domain.Names(project.EnvStrategies())})
	if q.Isolation.Origin == domain.OriginRequest {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: PathIsolation, Value: string(q.Isolation.Value), Accepted: domain.Names(project.Isolations())})...)
	}

	return problems
}

// Given is the request the questions answer, each field as settled.
func (q Questions) Given() Given {
	return Given{EnvFrom: q.EnvFrom.Value, Isolation: q.Isolation.Value, FastForward: new(q.FastForward.Value)}
}

// ----------------------------------------------------------------------------
// The answers

// Answers are the questions of a new worktree as settled, the only thing Run provisions a worktree from: a command cannot make one without asking them, and a question added here is asked by every command that does. Only Questions.Answers gives them.
type Answers struct {
	envFrom     project.EnvStrategy
	isolation   project.Isolation
	fastForward bool
}

// Answers are the questions as settled.
func (q Questions) Answers() Answers {
	return Answers{envFrom: q.EnvFrom.Value, isolation: q.Isolation.Value, fastForward: q.FastForward.Value}
}

// EnvFrom is where the worktree's .env files come from.
func (a Answers) EnvFrom() project.EnvStrategy {
	return a.envFrom
}

// Isolation is how the worktree stands against its source, as resolved: empty only when the worktree is not created.
func (a Answers) Isolation() project.Isolation {
	return a.isolation
}

// FastForward reports whether the source is fast-forwarded first.
func (a Answers) FastForward() bool {
	return a.fastForward
}

// Remembered are the answers of fields to remember, keyed by their id in config.toml.
func (a Answers) Remembered(fields []string) map[string]string {
	values := map[string]string{PathEnvFrom: string(a.envFrom), PathIsolation: string(a.isolation), PathFastForward: project.SourceUpdate(a.fastForward)}
	return project.RememberedAnswers(values, fields)
}
