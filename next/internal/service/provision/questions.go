package provision

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The request

// Given is what a request says to the questions of a new worktree, as given: a field left empty takes its remembered answer, its configured value or its default. EnvFrom may also be EnvFromConfig, which follows config.toml's strategy whatever is remembered. Remember names the questions whose answer is remembered for the next runs, Ask sets the remembered answers aside for this run. A command that creates a worktree embeds it in its request.
type Given struct {
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation"`
	FastForward *bool               `json:"fast_forward"` // nil when not given, so a remembered answer can apply
	Remember    []string            `json:"remember"`
	Ask         bool                `json:"ask"`
}

// Answer is the request with the field at path answered by values, spelled as the field's Answer spells them; a value the field cannot take, or a path that is not one of these questions, leaves the request as it was.
func (g Given) Answer(path string, values []string) (Given, error) {
	answer := domain.AnswerParams{Path: path, Values: values}
	next := g
	var err error
	switch path {
	case domain.PathRemember:
		next.Remember = values
	case domain.PathAsk:
		next.Ask, err = domain.Yes(answer)
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
	case domain.PathRemember:
		return g.Remember
	case domain.PathAsk:
		return domain.SaidYes(g.Ask)
	case PathEnvFrom:
		return domain.Spelled(g.EnvFrom)
	case PathIsolation:
		return domain.Spelled(g.Isolation)
	case PathFastForward:
		return domain.Said(g.FastForward)
	}

	return nil
}

// EnvFromConfig is the answer to the env strategy that follows config.toml's strategy, remembered as such.
const EnvFromConfig project.EnvStrategy = project.RememberEnvConfig

// The paths of the questions of a new worktree.
const (
	PathEnvFrom     = "env_from"
	PathIsolation   = "isolation"
	PathFastForward = "fast_forward"
)

// ----------------------------------------------------------------------------
// The questions

// Questions are the questions every command that creates a worktree asks: where its .env files come from (EnvFromConfig, or a strategy), how it stands against its source, and whether its source is fast-forwarded first; with config.toml's strategy, which EnvFromConfig follows, the fields whose answer is to be remembered and whether the remembered answers are set aside.
type Questions struct {
	EnvFrom     domain.Choice[project.EnvStrategy]
	Isolation   domain.Choice[project.Isolation]
	FastForward domain.Confirm
	Configured  project.EnvStrategy
	Remember    []string
	Ask         bool
}

// QuestionsParams is what the questions of a new worktree settle from: the request's answers, the repository (its remembered answers, which Given.Ask sets aside for the run while the form still shows them, its env strategy, its run.toml), why nothing is asked (Exists: the worktree is not created) and why the fast-forward is not (NotBehind: nothing it starts from is behind origin, worded by the command).
type QuestionsParams struct {
	Given      Given
	Repository worktree.Repository
	Exists     string
	NotBehind  string
}

// NewQuestions builds the questions of a new worktree: a field given keeps its value, an empty one takes its remembered answer, its configured value or its default; none is answered by default when the worktree is not created.
func NewQuestions(params QuestionsParams) Questions {
	recalled := params.Repository.Config.Remembered
	if params.Given.Ask {
		params.Repository.Config.Remembered = nil
	}

	questions := Questions{
		EnvFrom:     envFrom(params),
		Isolation:   isolation(params),
		FastForward: fastForward(params),
		Configured:  params.Repository.Config.Env.Strategy,
		Remember:    params.Given.Remember,
		Ask:         params.Given.Ask,
	}
	questions.EnvFrom.Remembered = project.EnvStrategy(recalled[project.RememberEnvStrategy])
	questions.Isolation.Remembered = project.Isolation(recalled[project.RememberIsolation])
	questions.FastForward.Remembered = domain.RememberedFastForward(recalled[project.RememberSourceUpdate])
	return questions
}

func envFrom(params QuestionsParams) domain.Choice[project.EnvStrategy] {
	question := domain.Choice[project.EnvStrategy]{Options: envFroms(), Skip: params.Exists}
	remembered := project.EnvStrategy(params.Repository.Config.Remembered[project.RememberEnvStrategy])
	switch {
	case params.Given.EnvFrom != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: params.Given.EnvFrom, Origin: domain.OriginRequest}
	case params.Exists != "":
	case remembered != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: remembered, Origin: domain.OriginRemembered}
	default:
		question.Answer = domain.Answer[project.EnvStrategy]{Value: EnvFromConfig, Origin: domain.OriginDefault}
	}

	return question
}

func isolation(params QuestionsParams) domain.Choice[project.Isolation] {
	question := domain.Choice[project.Isolation]{Options: project.Isolations()}
	switch {
	case params.Exists != "":
		question.Skip = params.Exists
	case !params.Repository.Run.IsolationApplies():
		question.Skip = "run.toml declares nothing a worktree isolates"
	}

	switch {
	case params.Given.Isolation != "":
		question.Answer = domain.Answer[project.Isolation]{Value: params.Given.Isolation, Origin: domain.OriginRequest}
	case params.Exists == "":
		question.Answer = domain.IsolationAnswer(project.Isolation(params.Repository.Config.Remembered[project.RememberIsolation]), params.Repository.Run)
	}

	return question
}

func fastForward(params QuestionsParams) domain.Confirm {
	return domain.FastForwardAnswer(domain.FastForwardParams{
		Given:      params.Given.FastForward,
		Remembered: params.Repository.Config.Remembered[project.RememberSourceUpdate],
		Skip:       cmp.Or(params.Exists, params.NotBehind),
	})
}

// envFroms are the answers to the env strategy, in the order the wizard offers them.
func envFroms() []project.EnvStrategy {
	return append([]project.EnvStrategy{EnvFromConfig}, project.EnvStrategies()...)
}

// Wording is how a command words the questions for its own subjects: the env strategy's help, the source update's help and its two options, the engine's own when zero.
type Wording struct {
	EnvHelp         string
	FastForwardHelp string
	Yes, No         domain.Option
}

// Fields are the questions as the fields of a form, in the order a wizard asks them, for a command to embed in its own.
func (q Questions) Fields(wording Wording) []domain.Field {
	return []domain.Field{
		domain.SelectField{
			FieldSpec:    domain.FieldSpec{Path: PathEnvFrom, Flag: "--env-from", Label: "Env strategy", Help: wording.EnvHelp, Origin: q.EnvFrom.Origin, Skip: q.EnvFrom.Skip},
			Value:        string(q.EnvFrom.Value),
			Options:      q.envOptions(),
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

// envOptions are the env strategies as the legacy wizard offers them, config.toml's first.
func (q Questions) envOptions() []domain.Option {
	return []domain.Option{
		{Value: string(EnvFromConfig), Label: "Use config default (" + string(q.Configured) + ")", Short: "config default"},
		{Value: string(project.EnvExample), Label: "example — copy .env.example → .env"},
		{Value: string(project.EnvMain), Label: "main — copy .env from the main checkout"},
		{Value: string(project.EnvParent), Label: "parent — copy .env from source worktree"},
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

// Problems are the problems of the answers as field errors: a strategy or, given, an isolation that is not one of those accepted, and a field to remember that is not one of these questions.
func (q Questions) Problems() []domain.FieldError {
	problems := domain.OneOf(domain.OneOfParams{Path: PathEnvFrom, Value: string(q.EnvFrom.Value), Accepted: domain.Names(envFroms())})
	if q.Isolation.Origin == domain.OriginRequest {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: PathIsolation, Value: string(q.Isolation.Value), Accepted: domain.Names(project.Isolations())})...)
	}

	for index, id := range q.Remember {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: domain.PathRemember + "[" + strconv.Itoa(index) + "]", Value: id, Accepted: rememberable()})...)
	}

	return problems
}

// rememberable are the questions whose answer can be remembered, in the order of the form.
func rememberable() []string {
	return []string{PathEnvFrom, PathIsolation, PathFastForward}
}

// Given is the request the questions answer, each field as settled; --ask is kept, so a request settled again sets the remembered answers aside as this one did.
func (q Questions) Given() Given {
	return Given{EnvFrom: q.EnvFrom.Value, Isolation: q.Isolation.Value, FastForward: new(q.FastForward.Value), Remember: q.remembered(), Ask: q.Ask}
}

// remembered are the fields whose answer is to be remembered, in the order of the form and each once; a field whose question is skipped is not.
func (q Questions) remembered() []string {
	var remembered []string
	for _, field := range rememberable() {
		if q.Remembers(field) {
			remembered = append(remembered, field)
		}
	}

	return remembered
}

// ----------------------------------------------------------------------------
// The answers

// Answers are the questions of a new worktree as settled, the only thing Run provisions a worktree from: a command cannot make one without asking them, and a question added here is asked by every command that does. Only Questions.Answers gives them; Run refuses the zero Answers.
type Answers struct {
	envFrom     project.EnvStrategy // as answered: EnvFromConfig, or a strategy
	strategy    project.EnvStrategy // the strategy it stands for
	isolation   project.Isolation
	fastForward bool
	asked       bool
}

// Answers are the questions as settled.
func (q Questions) Answers() Answers {
	strategy := q.EnvFrom.Value
	if strategy == EnvFromConfig {
		strategy = q.Configured
	}

	return Answers{envFrom: q.EnvFrom.Value, strategy: strategy, isolation: q.Isolation.Value, fastForward: q.FastForward.Value, asked: true}
}

// EnvFrom is where the worktree's .env files come from: the strategy EnvFromConfig stands for when the answer follows config.toml.
func (a Answers) EnvFrom() project.EnvStrategy {
	return a.strategy
}

// Isolation is how the worktree stands against its source, as resolved: empty only when the worktree is not created.
func (a Answers) Isolation() project.Isolation {
	return a.isolation
}

// Remembered are the answers of fields to remember, keyed by their id in config.toml.
func (a Answers) Remembered(fields []string) map[string]string {
	values := map[string][2]string{
		PathEnvFrom:     {project.RememberEnvStrategy, string(a.envFrom)},
		PathIsolation:   {project.RememberIsolation, string(a.isolation)},
		PathFastForward: {project.RememberSourceUpdate, project.SourceUpdate(a.fastForward)},
	}
	answers := map[string]string{}
	for _, field := range fields {
		if id, value := values[field][0], values[field][1]; id != "" && value != "" {
			answers[id] = value
		}
	}

	return answers
}
