package settings

import (
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// ----------------------------------------------------------------------------
// The request

// The paths of the settings, one per value config.toml holds.
const (
	PathBasePath    = "base_path"
	PathBaseBranch  = "base_branch"
	PathEnvStrategy = "env_strategy"
	PathEnvFiles    = "env_files"
	PathOnCreate    = "on_create"
	PathOnClean     = "on_clean"
)

// EnvNone is the env strategy that copies no .env file into a new worktree: config.toml then names neither a strategy nor a file.
const EnvNone project.EnvStrategy = "none"

// EnvStrategies are the answers to where a new worktree's .env files come from, in the order a choice offers them: each strategy, then none.
func EnvStrategies() []project.EnvStrategy {
	return append(project.EnvStrategies(), EnvNone)
}

// Given is what a request says to the settings, as given: a field left empty (nil for a list) takes the value config.toml holds, else what the repository suggests, else its default; a list given empty is none.
type Given struct {
	BasePath    string              `json:"base_path"`
	BaseBranch  string              `json:"base_branch"`
	EnvStrategy project.EnvStrategy `json:"env_strategy"`
	EnvFiles    []string            `json:"env_files"`
	OnCreate    []project.Hook      `json:"on_create"`
	OnClean     []project.Hook      `json:"on_clean"`
}

// Answer is the request with the setting at path answered by values, spelled as the setting's field spells them, a blank entry of a list left out; a path that is no setting leaves the request as it was.
func (g Given) Answer(path string, values []string) (Given, error) {
	next := g
	var err error
	switch path {
	case PathBasePath:
		next.BasePath, err = domain.One[string](path, values)
	case PathBaseBranch:
		next.BaseBranch, err = domain.One[string](path, values)
	case PathEnvStrategy:
		next.EnvStrategy, err = domain.One[project.EnvStrategy](path, values)
	case PathEnvFiles:
		next.EnvFiles = filled(values)
	case PathOnCreate:
		next.OnCreate = domain.HooksOf(filled(values))
	case PathOnClean:
		next.OnClean = domain.HooksOf(filled(values))
	default:
		err = domain.Unknown(path)
	}

	if err != nil {
		return g, err
	}
	return next, nil
}

// Values is the request's answer to the setting at path, spelled as Answer takes it; nil when the request leaves it to config.toml or its default, or path is no setting.
func (g Given) Values(path string) []string {
	switch path {
	case PathBasePath:
		return domain.Spelled(g.BasePath)
	case PathBaseBranch:
		return domain.Spelled(g.BaseBranch)
	case PathEnvStrategy:
		return domain.Spelled(g.EnvStrategy)
	case PathEnvFiles:
		return g.EnvFiles
	case PathOnCreate:
		return spelled(g.OnCreate)
	case PathOnClean:
		return spelled(g.OnClean)
	}

	return nil
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

func spelled(hooks []project.Hook) []string {
	if hooks == nil {
		return nil
	}
	values := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		values = append(values, domain.SpellHook(hook))
	}

	return values
}

// ----------------------------------------------------------------------------
// The questions

// Facts are what the settings are filled from: config.toml as written (nil when there is none), what the repository suggests (none for a read of the document), the branches the base branch can be, local then origin's, and the branch it suggests, origin's default else the main worktree's ("" when neither).
type Facts struct {
	Saved    *project.Saved
	Detected project.Detected
	Branches []string
	Head     string
}

// QuestionsParams is what the settings are settled from: the command that asks them, which names their problems, the request's answers and the facts.
type QuestionsParams struct {
	Command string
	Given   Given
	Facts   Facts
}

// Questions are the settings as a form, each answered as given, else as config.toml holds it, else as the repository suggests, else by default, with where its answer came from; config.toml's remembered answers, which a write keeps, and the file it would replace.
type Questions struct {
	BasePath    domain.Answer[string]
	BaseBranch  BaseBranch
	EnvStrategy domain.Answer[project.EnvStrategy]
	EnvFiles    EnvFiles
	OnCreate    Hooks
	OnClean     Hooks
	Dirs        []string
	command     string
	saved       *project.Saved
}

// BaseBranch is the question of the branch new worktrees start from: its answer, the branches it can be and the one the repository suggests, listed first.
type BaseBranch struct {
	domain.Answer[string]
	Options []string
	Pinned  string
}

// EnvFiles is the question of the .env files a new worktree gets: the targets picked, every one config.toml, the repository or the request knows, each with its template, and why it is not asked.
type EnvFiles struct {
	Picked  []string
	Origin  domain.Origin
	Options []project.EnvFile
	Skip    domain.Skip
}

// Hooks is the question of a list of hooks: its entries in the order they run, where they came from, and those the repository suggests.
type Hooks struct {
	Entries []project.Hook
	Origin  domain.Origin
	Found   []project.FoundHook
}

// NewQuestions settles each setting: a field given keeps its value; with a config.toml, the value it holds (a re-init shows the whole list the repository knows but picks only what config.toml holds, so what the user removed stays out); without one, what the repository suggests, else the default.
func NewQuestions(params QuestionsParams) Questions {
	given, facts := params.Given, params.Facts
	questions := Questions{
		BasePath:    basePath(given, facts.Saved),
		BaseBranch:  baseBranch(given, facts),
		EnvStrategy: envStrategy(given, facts.Saved),
		OnCreate:    hooks(hooksParams{Given: given.OnCreate, Saved: facts.Saved, Of: onCreate, Found: facts.Detected.OnCreate}),
		OnClean:     hooks(hooksParams{Given: given.OnClean, Saved: facts.Saved, Of: onClean, Found: facts.Detected.OnClean}),
		Dirs:        facts.Detected.Dirs,
		command:     params.Command,
		saved:       facts.Saved,
	}
	questions.EnvFiles = envFiles(given, facts, questions.EnvStrategy.Value)
	return questions
}

func basePath(given Given, saved *project.Saved) domain.Answer[string] {
	switch {
	case given.BasePath != "":
		return domain.Answer[string]{Value: given.BasePath, Origin: domain.OriginRequest}
	case saved != nil && saved.Config.Worktrees.BasePath != "":
		return domain.Answer[string]{Value: saved.Config.Worktrees.BasePath, Origin: domain.OriginConfig}
	}

	return domain.Answer[string]{Value: project.DefaultBasePath, Origin: domain.OriginDefault}
}

// baseBranch is the base branch as given, else as config.toml holds it, else the branch the repository suggests; none is a required field.
func baseBranch(given Given, facts Facts) BaseBranch {
	question := BaseBranch{Options: facts.Branches, Pinned: facts.Head}
	switch {
	case given.BaseBranch != "":
		question.Answer = domain.Answer[string]{Value: given.BaseBranch, Origin: domain.OriginRequest}
	case facts.Saved != nil && facts.Saved.Config.Worktrees.BaseBranch != "":
		question.Answer = domain.Answer[string]{Value: facts.Saved.Config.Worktrees.BaseBranch, Origin: domain.OriginConfig}
	case facts.Head != "":
		question.Answer = domain.Answer[string]{Value: facts.Head, Origin: domain.OriginDetected}
	default:
		question.Answer = domain.Answer[string]{Origin: domain.OriginDefault}
	}

	return question
}

// envStrategy is the strategy as given, else config.toml's: none when it names neither a strategy nor a file (as init writes a skipped .env), the default when it names files alone.
func envStrategy(given Given, saved *project.Saved) domain.Answer[project.EnvStrategy] {
	switch {
	case given.EnvStrategy != "":
		return domain.Answer[project.EnvStrategy]{Value: given.EnvStrategy, Origin: domain.OriginRequest}
	case saved == nil:
	case saved.Config.Env.Strategy != "":
		return domain.Answer[project.EnvStrategy]{Value: saved.Config.Env.Strategy, Origin: domain.OriginConfig}
	case len(saved.Config.Env.Files) == 0:
		return domain.Answer[project.EnvStrategy]{Value: EnvNone, Origin: domain.OriginConfig}
	}

	return domain.Answer[project.EnvStrategy]{Value: project.DefaultEnvStrategy, Origin: domain.OriginDefault}
}

// envFiles are the .env files picked as given, else those config.toml holds, else every one the repository holds; its options are the files config.toml holds, those the repository holds besides, then those given besides. None is asked when no .env file is copied, or there is none to pick.
func envFiles(given Given, facts Facts, strategy project.EnvStrategy) EnvFiles {
	question := EnvFiles{Options: envOptions(facts)}
	for _, target := range given.EnvFiles {
		if !slices.Contains(targets(question.Options), target) {
			question.Options = append(question.Options, project.EnvFile{Target: target})
		}
	}
	switch {
	case strategy == EnvNone:
		question.Skip = domain.SkipNoEnv
	case len(question.Options) == 0:
		question.Skip = domain.SkipNoEnvFile
	}
	switch {
	case given.EnvFiles != nil:
		question.Picked, question.Origin = given.EnvFiles, domain.OriginRequest
	case facts.Saved != nil:
		question.Picked, question.Origin = targets(facts.Saved.Config.Env.Files), domain.OriginConfig
	case len(facts.Detected.EnvFiles) > 0:
		question.Picked, question.Origin = targets(facts.Detected.EnvFiles), domain.OriginDetected
	default:
		question.Origin = domain.OriginDefault
	}

	return question
}

func envOptions(facts Facts) []project.EnvFile {
	var options []project.EnvFile
	if facts.Saved != nil {
		options = slices.Clone(facts.Saved.Config.Env.Files)
	}
	for _, file := range facts.Detected.EnvFiles {
		if !slices.Contains(targets(options), file.Target) {
			options = append(options, file)
		}
	}

	return options
}

func targets(files []project.EnvFile) []string {
	found := make([]string, 0, len(files))
	for _, file := range files {
		found = append(found, file.Target)
	}

	return found
}

// hooksParams is what a list of hooks settles from: the request's, config.toml's (Of picks which list), and those the repository suggests.
type hooksParams struct {
	Given []project.Hook
	Saved *project.Saved
	Of    func(project.Config) []project.Hook
	Found []project.FoundHook
}

func onCreate(config project.Config) []project.Hook { return config.OnCreate }
func onClean(config project.Config) []project.Hook  { return config.OnClean }

// hooks is a list of hooks as given, else as config.toml holds it, else as the repository suggests.
func hooks(params hooksParams) Hooks {
	question := Hooks{Found: params.Found}
	switch {
	case params.Given != nil:
		question.Entries, question.Origin = params.Given, domain.OriginRequest
	case params.Saved != nil:
		question.Entries, question.Origin = params.Of(params.Saved.Config), domain.OriginConfig
	case len(params.Found) > 0:
		for _, found := range params.Found {
			question.Entries = append(question.Entries, found.Hook)
		}
		question.Origin = domain.OriginDetected
	default:
		question.Origin = domain.OriginDefault
	}

	return question
}

// Fields are the settings as the fields of a form, in the order a wizard asks them, for a command to embed in its own.
func (q Questions) Fields() []domain.Field {
	branches := make([]domain.Option, 0, len(q.BaseBranch.Options))
	for _, branch := range q.BaseBranch.Options {
		branches = append(branches, domain.Option{Value: branch})
	}
	files := make([]domain.Option, 0, len(q.EnvFiles.Options))
	for _, file := range q.EnvFiles.Options {
		files = append(files, domain.Option{Value: file.Target, Template: file.Template})
	}

	return []domain.Field{
		domain.TextField{FieldSpec: domain.FieldSpec{Path: PathBasePath, Origin: q.BasePath.Origin}, Value: q.BasePath.Value},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathBaseBranch, Origin: q.BaseBranch.Origin}, Value: q.BaseBranch.Value, Options: branches, Pinned: q.BaseBranch.Pinned},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathEnvStrategy, Origin: q.EnvStrategy.Origin}, Value: string(q.EnvStrategy.Value), Options: domain.OptionsOf(EnvStrategies()), Closed: true},
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: PathEnvFiles, Origin: q.EnvFiles.Origin, Skip: q.EnvFiles.Skip}, Picked: q.EnvFiles.Picked, Options: files},
		domain.HooksField{FieldSpec: domain.FieldSpec{Path: PathOnCreate, Origin: q.OnCreate.Origin}, Entries: q.OnCreate.Entries, Found: q.OnCreate.Found, Dirs: q.Dirs},
		domain.HooksField{FieldSpec: domain.FieldSpec{Path: PathOnClean, Origin: q.OnClean.Origin}, Entries: q.OnClean.Entries, Found: q.OnClean.Found, Dirs: q.Dirs},
	}
}

// Given is the request the questions answer, each setting as settled.
func (q Questions) Given() Given {
	return Given{
		BasePath:    q.BasePath.Value,
		BaseBranch:  q.BaseBranch.Value,
		EnvStrategy: q.EnvStrategy.Value,
		EnvFiles:    answered(q.EnvFiles.Picked),
		OnCreate:    answered(q.OnCreate.Entries),
		OnClean:     answered(q.OnClean.Entries),
	}
}

// answered is list as an answer: empty rather than nil, since nil leaves a list to its default.
func answered[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}

// ----------------------------------------------------------------------------
// The problems

// The codes of a setting's problem, each after the asking command's name and a dot.
const (
	CodeBasePathAbsolute  = "base_path_absolute"
	CodeBaseBranchUnknown = "base_branch_unknown"
	CodeEnvFileOutside    = "env_file_outside"
	CodeEnvFileTwice      = "env_file_twice"
	CodeHookDirOutside    = "hook_dir_outside"
)

// Problems are the settings' problems as field errors, each coded after the asking command: a base path that is absolute; no base branch, or one that is no branch of the repository; an env strategy that is none of those accepted; an .env file outside the repository or listed twice; a hook that runs outside the repository.
func (q Questions) Problems() []domain.FieldError {
	var problems []domain.FieldError
	if path.IsAbs(q.BasePath.Value) {
		problems = append(problems, q.problem(PathBasePath, CodeBasePathAbsolute, "{path} must be relative to the repository, not {value}", q.BasePath.Value))
	}
	switch branch := q.BaseBranch.Value; {
	case branch == "":
		problems = append(problems, domain.Required(PathBaseBranch))
	case len(q.BaseBranch.Options) > 0 && !slices.Contains(q.BaseBranch.Options, branch):
		problems = append(problems, q.problem(PathBaseBranch, CodeBaseBranchUnknown, "{path} {value} is no branch of the repository", branch))
	}
	problems = append(problems, domain.OneOf(domain.OneOfParams{Path: PathEnvStrategy, Value: string(q.EnvStrategy.Value), Accepted: domain.Names(EnvStrategies())})...)

	if q.EnvStrategy.Value != EnvNone {
		for index, target := range q.EnvFiles.Picked {
			at := PathEnvFiles + "[" + strconv.Itoa(index) + "]"
			switch {
			case outside(target):
				problems = append(problems, q.problem(at, CodeEnvFileOutside, "{path} must be a file of the repository, not {value}", target))
			case slices.Index(q.EnvFiles.Picked, target) < index:
				problems = append(problems, q.problem(at, CodeEnvFileTwice, "{path} {value} is listed twice", target))
			}
		}
	}

	problems = append(problems, q.hookProblems(PathOnCreate, q.OnCreate.Entries)...)
	return append(problems, q.hookProblems(PathOnClean, q.OnClean.Entries)...)
}

func (q Questions) hookProblems(at string, hooks []project.Hook) []domain.FieldError {
	var problems []domain.FieldError
	for index, hook := range hooks {
		if hook.Cwd != "" && outside(hook.Cwd) {
			problems = append(problems, q.problem(at+"["+strconv.Itoa(index)+"]", CodeHookDirOutside, "{path} must run in a folder of the repository, not {value}", hook.Cwd))
		}
	}

	return problems
}

func (q Questions) problem(at, code, template, value string) domain.FieldError {
	return domain.FieldError{Path: at, Code: q.command + "." + code, Template: template, Params: domain.Params{"value": value}}
}

// outside reports whether a path, relative to the repository, leads out of it.
func outside(at string) bool {
	clean := path.Clean(at)
	return path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../")
}

// ----------------------------------------------------------------------------
// The document

// Rewrite is config.toml as the settings settle it, to replace the file they were read from: a strategy of none names neither a strategy nor a file, each .env file keeps the template config.toml or the repository gives it, and the remembered answers are kept as they are.
func (q Questions) Rewrite() project.Rewrite {
	config := project.Config{
		Worktrees: project.Worktrees{BasePath: q.BasePath.Value, BaseBranch: q.BaseBranch.Value},
		OnCreate:  q.OnCreate.Entries,
		OnClean:   q.OnClean.Entries,
	}
	if q.EnvStrategy.Value != EnvNone {
		config.Env.Strategy = q.EnvStrategy.Value
		for _, target := range q.EnvFiles.Picked {
			config.Env.Files = append(config.Env.Files, q.envFile(target))
		}
	}

	rewrite := project.Rewrite{Config: config}
	if q.saved != nil {
		rewrite.Config.Remembered, rewrite.Replaces = q.saved.Config.Remembered, q.saved.Digest
	}
	return rewrite
}

// envFile is the file target stands for: as config.toml or the repository know it, else with no template, looked up beside it when copied.
func (q Questions) envFile(target string) project.EnvFile {
	if at := slices.IndexFunc(q.EnvFiles.Options, func(file project.EnvFile) bool { return file.Target == target }); at >= 0 {
		return q.EnvFiles.Options[at]
	}

	return project.EnvFile{Target: target}
}
