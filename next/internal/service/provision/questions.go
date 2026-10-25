package provision

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

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
	next := g
	var err error
	switch path {
	case domain.PathRemember:
		next.Remember = values
	case domain.PathAsk:
		next.Ask, err = domain.Yes(path, values)
	case domain.PathEnvFrom:
		next.EnvFrom, err = domain.One[project.EnvStrategy](path, values)
	case domain.PathIsolation:
		next.Isolation, err = domain.One[project.Isolation](path, values)
	case domain.PathFastForward:
		var yes bool
		yes, err = domain.Yes(path, values)
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
	case domain.PathEnvFrom:
		return domain.Spelled(g.EnvFrom)
	case domain.PathIsolation:
		return domain.Spelled(g.Isolation)
	case domain.PathFastForward:
		return domain.Said(g.FastForward)
	}

	return nil
}

// EnvFromConfig is the answer to the env strategy that follows config.toml's strategy, remembered as such.
const EnvFromConfig project.EnvStrategy = project.RememberEnvConfig

// ----------------------------------------------------------------------------
// The questions

// Questions are the questions every command that creates a worktree asks: where its .env files come from (EnvFromConfig, or a strategy), how it stands against its source, and whether its source is fast-forwarded first; with config.toml's strategy, which EnvFromConfig follows, the branches the fast-forward updates, whether several worktrees are created, the fields whose answer is to be remembered and whether the remembered answers are set aside.
type Questions struct {
	EnvFrom     domain.Choice[project.EnvStrategy]
	Isolation   domain.Choice[project.Isolation]
	FastForward domain.Confirm
	Configured  project.EnvStrategy
	Behind      []string
	Many        bool
	Remember    []string
	Ask         bool
	runUnread   bool // the isolation is skipped only because run.toml could not be read, and settles the worktree all the same
}

// QuestionsParams is what the questions of a new worktree settle from: the request's answers, the repository (its remembered answers, which Given.Ask sets aside for the run while the form still shows them, its env strategy, its run.toml), why nothing is asked (Exists: the worktree is not created), the branches behind origin the fast-forward would update (none: it is not asked) and whether several worktrees are created.
type QuestionsParams struct {
	Given      Given
	Repository worktree.Repository
	Exists     string
	Behind     []string
	Held       string // why a branch behind origin is not fast-forwarded all the same, set by the command
	Many       bool
}

const (
	isolationHelp   = "How it stands against its source: isolated or verbatim; run.toml's isolation by default"
	fastForwardHelp = "Fast-forward the source branch to origin first when it is behind"
	notBehind       = "nothing to fast-forward: the source is not behind origin"
)

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
		Behind:      params.Behind,
		Many:        params.Many,
		Remember:    params.Given.Remember,
		Ask:         params.Given.Ask,
		runUnread:   params.Exists == "" && params.Repository.RunErr != nil,
	}
	questions.EnvFrom.Remembered = project.EnvStrategy(recalled[project.RememberEnvStrategy])
	questions.Isolation.Remembered = project.Isolation(recalled[project.RememberIsolation])
	questions.FastForward.Remembered = rememberedFastForward(recalled[project.RememberSourceUpdate])
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
	case params.Repository.RunErr != nil:
		question.Skip = "run.toml could not be read"
	case !params.Repository.Run.IsolationApplies():
		question.Skip = "run.toml declares nothing a worktree isolates"
	}

	switch {
	case params.Given.Isolation != "":
		question.Answer = domain.Answer[project.Isolation]{Value: params.Given.Isolation, Origin: domain.OriginRequest}
	case params.Exists == "":
		question.Answer = isolationAnswer(project.Isolation(params.Repository.Config.Remembered[project.RememberIsolation]), params.Repository)
	}

	return question
}

// isolationAnswer is the isolation a question the request leaves unanswered settles on: the remembered one (empty when none) while run.toml declares something a worktree isolates or could not be read, else run.toml's, else isolated.
func isolationAnswer(remembered project.Isolation, repo worktree.Repository) domain.Answer[project.Isolation] {
	run := repo.Run
	switch {
	case remembered != "" && (run.IsolationApplies() || repo.RunErr != nil):
		return domain.Answer[project.Isolation]{Value: remembered, Origin: domain.OriginRemembered}
	case run.Isolation != "":
		return domain.Answer[project.Isolation]{Value: run.Isolation.Effective(), Origin: domain.OriginConfig}
	}

	return domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}
}

// fastForward is the question whether to fast-forward the source first: the request's answer, none while the question is skipped, else the remembered one, else no.
func fastForward(params QuestionsParams) domain.Confirm {
	remembered := params.Repository.Config.Remembered[project.RememberSourceUpdate]
	question := domain.Confirm{Skip: cmp.Or(params.Exists, params.Held)}
	if question.Skip == "" && len(params.Behind) == 0 {
		question.Skip = notBehind
	}
	switch {
	case params.Given.FastForward != nil:
		question.Answer = domain.Answer[bool]{Value: *params.Given.FastForward, Origin: domain.OriginRequest}
	case question.Skip != "":
	case remembered != "":
		question.Answer = domain.Answer[bool]{Value: remembered == project.SourceUpdateFastForward, Origin: domain.OriginRemembered}
	default:
		question.Answer = domain.Answer[bool]{Origin: domain.OriginDefault}
	}

	return question
}

// rememberedFastForward is the answer a remembered source update gives, nil when none is.
func rememberedFastForward(remembered string) *bool {
	if remembered == "" {
		return nil
	}

	return new(remembered == project.SourceUpdateFastForward)
}

// envFroms are the answers to the env strategy, in the order the wizard offers them.
func envFroms() []project.EnvStrategy {
	return append([]project.EnvStrategy{EnvFromConfig}, project.EnvStrategies()...)
}

// Fields are the questions as the fields of a form, in the order a wizard asks them, for a command to embed in its own; envHelp says where the command's new worktree takes its .env files from.
func (q Questions) Fields(envHelp string) []domain.Field {
	return []domain.Field{
		domain.SelectField{
			FieldSpec:    domain.FieldSpec{Path: domain.PathEnvFrom, Flag: "--env-from", Label: "Env strategy", Help: envHelp, Origin: q.EnvFrom.Origin, Skip: q.EnvFrom.Skip},
			Value:        string(q.EnvFrom.Value),
			Options:      q.envOptions(),
			Remembered:   string(q.EnvFrom.Remembered),
			Rememberable: true,
			Remember:     q.Remembers(domain.PathEnvFrom),
		},
		domain.SelectField{
			FieldSpec:    domain.FieldSpec{Path: domain.PathIsolation, Flag: "--isolation", Label: "Isolation", Help: isolationHelp, Origin: q.Isolation.Origin, Skip: q.Isolation.Skip},
			Value:        string(q.Isolation.Value),
			Options:      q.isolationOptions(),
			Remembered:   string(q.Isolation.Remembered),
			Rememberable: true,
			Remember:     q.Remembers(domain.PathIsolation),
		},
		domain.ConfirmField{
			FieldSpec:    domain.FieldSpec{Path: domain.PathFastForward, Flag: "--ff", Label: "Source update", Help: fastForwardHelp, Origin: q.FastForward.Origin, Skip: q.FastForward.Skip},
			Value:        q.FastForward.Value,
			Yes:          domain.Option{Value: "true", Label: q.fastForwardLabel(), Short: "fast-forward to origin"},
			No:           domain.Option{Value: "false", Label: "Keep it as-is", Short: "keep as-is"},
			Remembered:   q.FastForward.Remembered,
			Rememberable: true,
			Remember:     q.Remembers(domain.PathFastForward),
		},
	}
}

func (q Questions) fastForwardLabel() string {
	if len(q.Behind) == 0 {
		return "Fast-forward to origin"
	}

	return "Fast-forward " + strings.Join(q.Behind, ", ") + " to origin"
}

// isolationOptions are the isolations as the legacy wizard offers them, worded for one worktree or several.
func (q Questions) isolationOptions() []domain.Option {
	if q.Many {
		return []domain.Option{
			{Value: string(project.Isolated), Label: "isolated — each its own ports and namespaces"},
			{Value: string(project.Verbatim), Label: "verbatim — the copied .env as is, one runs at a time"},
		}
	}

	return []domain.Option{
		{Value: string(project.Isolated), Label: "isolated — its own ports and namespaces"},
		{Value: string(project.Verbatim), Label: "verbatim — the copied .env as is, one runs at a time"},
	}
}

// envOptions are the env strategies as the legacy wizard offers them, config.toml's first.
func (q Questions) envOptions() []domain.Option {
	return []domain.Option{
		{Value: string(EnvFromConfig), Label: "config default (" + string(q.Configured) + ")", Short: "config default"},
		{Value: string(project.EnvExample), Label: "example — from .env.example"},
		{Value: string(project.EnvMain), Label: "main — the main checkout's .env"},
		{Value: string(project.EnvParent), Label: "parent — the source worktree's .env"},
	}
}

// Remembers reports whether the answer to the field at path is to be remembered: asked to be, and its question not skipped, unless the isolation is skipped only because run.toml could not be read, which settles the worktree all the same.
func (q Questions) Remembers(path string) bool {
	settles := !q.skipped(path) || path == domain.PathIsolation && q.runUnread
	return slices.Contains(q.Remember, path) && settles
}

func (q Questions) skipped(path string) bool {
	switch path {
	case domain.PathEnvFrom:
		return q.EnvFrom.Skip != ""
	case domain.PathIsolation:
		return q.Isolation.Skip != ""
	case domain.PathFastForward:
		return q.FastForward.Skip != ""
	}

	return false
}

// Problems are the problems of the answers as field errors: a strategy or, given, an isolation that is not one of those accepted, and a field to remember that is not one of these questions.
func (q Questions) Problems() []domain.FieldError {
	problems := domain.OneOf(domain.OneOfParams{Path: domain.PathEnvFrom, Value: string(q.EnvFrom.Value), Accepted: domain.Names(envFroms())})
	if q.Isolation.Origin == domain.OriginRequest {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: domain.PathIsolation, Value: string(q.Isolation.Value), Accepted: domain.Names(project.Isolations())})...)
	}

	for index, id := range q.Remember {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: domain.PathRemember + "[" + strconv.Itoa(index) + "]", Value: id, Accepted: rememberable()})...)
	}

	return problems
}

// rememberable are the questions whose answer can be remembered, in the order of the form.
func rememberable() []string {
	return []string{domain.PathEnvFrom, domain.PathIsolation, domain.PathFastForward}
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

// Recalled are the fields answered from memory, in the order of the form.
func (q Questions) Recalled() []string {
	origins := map[string]domain.Origin{domain.PathEnvFrom: q.EnvFrom.Origin, domain.PathIsolation: q.Isolation.Origin, domain.PathFastForward: q.FastForward.Origin}
	var recalled []string
	for _, field := range rememberable() {
		// by its origin, not its skip: a remembered isolation still settles a worktree whose run.toml could not be read
		if origins[field] == domain.OriginRemembered {
			recalled = append(recalled, field)
		}
	}

	return recalled
}

// FollowsConfig reports whether the env strategy follows config.toml's (EnvFromConfig) rather than naming one.
func (a Answers) FollowsConfig() bool {
	return a.envFrom == EnvFromConfig
}

// EnvFrom is where the worktree's .env files come from: the strategy EnvFromConfig stands for when the answer follows config.toml.
func (a Answers) EnvFrom() project.EnvStrategy {
	return a.strategy
}

// Isolation is how the worktree stands against its source, as resolved: empty only when the worktree is not created.
func (a Answers) Isolation() project.Isolation {
	return a.isolation
}

// Remembered are the answers of fields to remember, keyed by their id in config.toml, and those fields, a field without an answer left out.
func (a Answers) Remembered(fields []string) (map[string]string, []string) {
	values := map[string][2]string{
		domain.PathEnvFrom:     {project.RememberEnvStrategy, string(a.envFrom)},
		domain.PathIsolation:   {project.RememberIsolation, string(a.isolation)},
		domain.PathFastForward: {project.RememberSourceUpdate, project.SourceUpdate(a.fastForward)},
	}
	answers := map[string]string{}
	var kept []string
	for _, field := range fields {
		if id, value := values[field][0], values[field][1]; id != "" && value != "" {
			answers[id] = value
			kept = append(kept, field)
		}
	}

	return answers, kept
}
