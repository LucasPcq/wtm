package env

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	envs "github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of env: the worktrees it names, each by its branch or its path, else the one it stands in, or every worktree with All; the strategy the values come from for this run (Strategy), empty for the one each worktree was created with; whether a key holding another value than its source's takes the source's (Overwrite), and whether a key in no source is dropped (Prune); the isolation each linked worktree asked switches to, empty to keep the one it has; and whether the values wtm does not write are shown (ShowValues). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees  []string            `json:"worktree"`
	All        bool                `json:"all"`
	Strategy   project.EnvStrategy `json:"from"`
	Overwrite  bool                `json:"overwrite"`
	Prune      bool                `json:"prune"`
	Isolation  project.Isolation   `json:"isolation"`
	ShowValues bool                `json:"show_values"`
	Caller     domain.Caller       `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of env's own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathAll        = "all"
	PathFrom       = "from"
	PathOverwrite  = "overwrite"
	PathPrune      = "prune"
	PathShowValues = "show_values"
)

// Answer is the request with the field at path answered by values. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktrees = values
	case PathAll:
		next.All, err = domain.Yes(path, values)
	case PathFrom:
		next.Strategy, err = domain.One[project.EnvStrategy](path, values)
	case PathOverwrite:
		next.Overwrite, err = domain.Yes(path, values)
	case PathPrune:
		next.Prune, err = domain.Yes(path, values)
	case domain.PathIsolation:
		next.Isolation, err = domain.One[project.Isolation](path, values)
	case PathShowValues:
		next.ShowValues, err = domain.Yes(path, values)
	default:
		return r, domain.Unknown(path)
	}

	if err != nil {
		return r, err
	}
	return next, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves the field to its default.
func (r Request) Values(path string) []string {
	switch path {
	case domain.PathWorktree:
		return r.Worktrees
	case PathAll:
		return domain.SaidYes(r.All)
	case PathFrom:
		return domain.Spelled(r.Strategy)
	case PathOverwrite:
		return domain.SaidYes(r.Overwrite)
	case PathPrune:
		return domain.SaidYes(r.Prune)
	case domain.PathIsolation:
		return domain.Spelled(r.Isolation)
	case PathShowValues:
		return domain.SaidYes(r.ShowValues)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees to reconcile and those it may, whether every one is, the strategy for this run, whether to overwrite the conflicts and prune the orphans of the worktrees asked (the keys of each), each asked only when they have some, the isolation to switch them to, asked only when one has yet to choose it, whether to show the values, and the request's problems as field errors.
type Questions struct {
	Worktrees  Worktrees
	All        bool
	From       project.EnvStrategy
	Overwrite  domain.Confirm
	Prune      domain.Confirm
	Conflicts  []KeyAt
	Orphans    []KeyAt
	Isolation  domain.Choice[project.Isolation]
	ShowValues bool
	Errors     []domain.FieldError
}

// Worktrees is the question of the worktrees to reconcile: those given, else its default pick, the one the caller stands in, and the pick a person is offered, the worktrees out of step and the caller's; and every worktree on a branch, the main one first, each with the changes reconciling it makes and its isolation.
type Worktrees struct {
	Picked     domain.Answer[[]string]
	Prechecked []string
	Options    []domain.Option
}

// NewQuestions builds the questions for the request on the facts: the worktrees given, else the default pick; whether to overwrite the conflicts, asked when a worktree asked has one, and whether to prune the orphans, asked when one has one.
func NewQuestions(f Facts, req Request) Questions {
	reconciles, problems := f.entries(req)
	questions := Questions{
		Worktrees:  Worktrees{Picked: domain.Answer[[]string]{Value: req.Worktrees, Origin: domain.OriginRequest}, Options: f.worktreeOptions(req)},
		All:        req.All,
		From:       req.Strategy,
		ShowValues: req.ShowValues,
		Errors:     append(append(problems, f.fromProblems(req)...), f.isolationProblems(req, reconciles)...),
	}
	if len(req.Worktrees) == 0 && !req.All {
		questions.Worktrees.Picked = domain.Answer[[]string]{Value: names(f.targets(req)), Origin: domain.OriginDefault}
		questions.Worktrees.Prechecked = f.prechecked(req, questions.Worktrees.Options)
	}
	questions.Conflicts, questions.Orphans = keysOf(reconciles, envs.KeyConflict), keysOf(reconciles, envs.KeyOrphan)
	questions.Overwrite = confirm(req.Overwrite, len(questions.Conflicts) > 0, domain.SkipNoKeyConflict)
	questions.Prune = confirm(req.Prune, len(questions.Orphans) > 0, domain.SkipNoKeyOrphan)
	questions.Isolation = domain.Choice[project.Isolation]{Answer: domain.Answer[project.Isolation]{Value: req.Isolation, Origin: domain.OriginRequest}, Options: []project.Isolation{project.Isolated, project.Verbatim}}
	switch {
	case req.Isolation == "" && slices.ContainsFunc(reconciles, func(r Reconcile) bool { return r.Pending }):
		questions.Isolation.Answer = domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}
	case req.Isolation == "":
		questions.Isolation.Origin, questions.Isolation.Skip = domain.OriginDefault, domain.SkipFlagOnly
	}

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, unless every one is reconciled, then whether every one is and the strategy, switches only their flags set, whether to overwrite the conflicts and prune the orphans, then whether to show the values, a switch only its flag sets. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	worktrees := domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktrees.Picked.Origin}
	if q.All {
		worktrees.Skip = domain.SkipAll
	}

	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: worktrees, Picked: q.Worktrees.Picked.Value, Prechecked: q.Worktrees.Prechecked, Options: q.Worktrees.Options},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathAll, Skip: domain.SkipFlagOnly}, Value: q.All, NoFirst: true},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathFrom, Skip: domain.SkipFlagOnly}, Value: string(q.From), Options: strategyOptions(), Closed: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathOverwrite, Origin: q.Overwrite.Origin, Skip: q.Overwrite.Skip}, Value: q.Overwrite.Value, NoFirst: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathPrune, Origin: q.Prune.Origin, Skip: q.Prune.Skip}, Value: q.Prune.Value, NoFirst: true},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: domain.PathIsolation, Origin: q.Isolation.Origin, Skip: q.Isolation.Skip}, Value: string(q.Isolation.Value), Options: isolationOptions(q.Isolation.Options), Closed: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathShowValues, Skip: domain.SkipFlagOnly}, Value: q.ShowValues, NoFirst: true},
	}}
}

func isolationOptions(isolations []project.Isolation) []domain.Option {
	options := make([]domain.Option, 0, len(isolations))
	for _, isolation := range isolations {
		options = append(options, domain.Option{Value: string(isolation)})
	}
	return options
}

func strategyOptions() []domain.Option {
	options := make([]domain.Option, 0, len(project.EnvStrategies()))
	for _, strategy := range project.EnvStrategies() {
		options = append(options, domain.Option{Value: string(strategy)})
	}
	return options
}

// worktreeOptions are the worktrees env may reconcile: each on a branch, the main one first, tagged as every picker tags them from where the caller stands, with the changes reconciling each makes under req's switches and its isolation when it is not isolated.
func (f Facts) worktreeOptions(req Request) []domain.Option {
	options := domain.WorktreeOptions(f.onBranch(), domain.WorktreeQuery{Cwd: req.Caller.Cwd})
	for i, option := range options {
		r := f.reconcile(option.Value, req)
		options[i].OutOfStep = r.changes()
		if r.Isolation != project.Isolated {
			options[i].Isolation = r.Isolation
		}
	}

	return options
}

// confirm is a switch whose no is its default: given, it stands as the request's; else no, its question skipped with skip unless asked.
func confirm(given, asked bool, skip domain.Skip) domain.Confirm {
	switch {
	case given:
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	case asked:
		return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}}
	}

	return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: skip}
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	return req, nil
}

// named reports whether the request names its worktrees, the user's choice, rather than leaving them to the command.
func (r Request) named() bool {
	return len(r.Worktrees) > 0 && !r.All
}

// CodeAllAndNames refuses every worktree asked with some named; CodeDetached refuses a worktree on no branch, whose record and addresses its branch names; CodeMainIsolated refuses an isolation other than isolated for the main checkout, the one the others copy from.
const (
	CodeAllAndNames  = Name + ".all_and_names"
	CodeDetached     = Name + ".detached"
	CodeMainIsolated = Name + ".main_isolated"
)

// isolationProblems refuses an isolation that is none of the isolations, and one other than isolated when the main checkout is asked.
func (f Facts) isolationProblems(req Request, reconciles []Reconcile) []domain.FieldError {
	if req.Isolation == "" {
		return nil
	}
	if problems := domain.OneOf(domain.OneOfParams{Path: domain.PathIsolation, Value: string(req.Isolation), Accepted: domain.Names([]project.Isolation{project.Isolated, project.Verbatim})}); len(problems) > 0 {
		return problems
	}
	if req.Isolation != project.Isolated && slices.ContainsFunc(reconciles, func(r Reconcile) bool { return r.Main }) {
		return []domain.FieldError{{Path: domain.PathIsolation, Code: CodeMainIsolated, Template: "the main checkout is always isolated: it is the one the others copy from", Params: domain.Params{"value": string(req.Isolation)}}}
	}
	return nil
}

// fromProblems refuses a strategy that is none of the strategies.
func (f Facts) fromProblems(req Request) []domain.FieldError {
	if req.Strategy == "" {
		return nil
	}
	return domain.OneOf(domain.OneOfParams{Path: PathFrom, Value: string(req.Strategy), Accepted: domain.Names(project.EnvStrategies())})
}

// entries are the worktrees req asks, reconciled: those it names in the order asked, else the default pick, or every one on a branch with All, the main first; and the problem of each entry it cannot take: blank, given twice, a worktree on no branch, a name that is no worktree, a name with All.
func (f Facts) entries(req Request) (reconciles []Reconcile, problems []domain.FieldError) {
	if req.All && len(req.Worktrees) > 0 {
		problems = append(problems, domain.FieldError{Path: PathAll, Code: CodeAllAndNames, Template: "{path} reconciles every worktree: name none with it"})
	}
	if !req.named() {
		for _, w := range f.targets(req) {
			reconciles = append(reconciles, f.reconcile(w.Name(), req))
		}
		return reconciles, problems
	}

	for index, name := range req.Worktrees {
		path := entryPath(index)
		w, found := f.subject(name)
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(req.Worktrees[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case !found:
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": name}})
		case w.Branch() == "":
			problems = append(problems, domain.FieldError{Path: path, Code: CodeDetached, Template: "{value} is on no branch: env reconciles a worktree on a branch", Params: domain.Params{"value": name}})
		case slices.ContainsFunc(reconciles, func(r Reconcile) bool { return r.Worktree == w.Name() }):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} names a worktree given already", Params: domain.Params{"value": name}})
		default:
			reconciles = append(reconciles, f.reconcile(w.Name(), req))
		}
	}

	return reconciles, problems
}

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}
