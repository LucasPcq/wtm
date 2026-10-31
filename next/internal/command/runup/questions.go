package runup

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of run up: the worktrees to start a profile in, each by its branch or, on no branch, by its path (none: the one the caller stands in); the Profile of run.toml to start (none: its default); whether the other worktrees' jobs are stopped first (Exclusive, nil leaving it to run.toml, else to the question), or kept (Parallel, which says no to Exclusive); whether a job that changes data the worktree does not own starts all the same (Force); whether the ports the jobs declare go unchecked (NoProbe); and whether the command line returns once they are started rather than show them (Detach). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	Profile   string        `json:"profile"`
	Exclusive *bool         `json:"exclusive"`
	Parallel  bool          `json:"parallel"`
	Force     bool          `json:"force"`
	NoProbe   bool          `json:"no_probe"`
	Detach    bool          `json:"detach"`
	Caller    domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of run up's own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathProfile   = "profile"
	PathExclusive = "exclusive"
	PathParallel  = "parallel"
	PathForce     = "force"
	PathNoProbe   = "no_probe"
	PathDetach    = "detach"
)

// Answer is the request with the field at path answered by values. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktrees = values
	case PathProfile:
		next.Profile = strings.Join(values, "")
	case PathExclusive:
		var yes bool
		yes, err = domain.Yes(path, values)
		next.Exclusive = &yes
	case PathParallel:
		next.Parallel, err = domain.Yes(path, values)
	case PathForce:
		next.Force, err = domain.Yes(path, values)
	case PathNoProbe:
		next.NoProbe, err = domain.Yes(path, values)
	case PathDetach:
		next.Detach, err = domain.Yes(path, values)
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
	case PathProfile:
		if r.Profile == "" {
			return nil
		}
		return []string{r.Profile}
	case PathExclusive:
		return domain.Said(r.Exclusive)
	case PathParallel:
		return domain.SaidYes(r.Parallel)
	case PathForce:
		return domain.SaidYes(r.Force)
	case PathNoProbe:
		return domain.SaidYes(r.NoProbe)
	case PathDetach:
		return domain.SaidYes(r.Detach)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees and those it may start in, the profile and those run.toml declares, whether the others' jobs are stopped first, whether a job that changes foreign data starts all the same, and the request's problems as field errors.
type Questions struct {
	Worktrees Worktrees
	Profile   domain.Choice[string]
	Profiles  []domain.Option
	Exclusive domain.Confirm
	Force     domain.Confirm
	Request   Request
	Errors    []domain.FieldError
}

// Worktrees is the question of the worktrees to start in: those picked, the request's or by default the one the caller stands in, and every one that stands, each with the jobs it runs.
type Worktrees struct {
	Picked  []string
	Origin  domain.Origin
	Options []domain.Option
}

// NewQuestions builds the questions for the request on the facts.
func NewQuestions(f Facts, req Request) Questions {
	picked, origin := f.picked(req)
	targets, problems := f.entries(picked)
	questions := Questions{
		Worktrees: Worktrees{Picked: picked, Origin: origin, Options: f.worktreeOptions()},
		Profile:   f.profile(req),
		Profiles:  f.profileOptions(),
		Request:   req,
		Errors:    problems,
	}
	if len(picked) == 0 {
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	questions.Errors = append(questions.Errors, f.profileProblems(req)...)

	exclusive := f.exclusive(req, targets)
	questions.Exclusive = exclusive.Confirm
	questions.Errors = append(questions.Errors, exclusive.problems...)

	risky := len(f.risks(targets, f.jobsOf(questions.Profile.Value))) > 0
	questions.Force = domain.Confirm{Answer: domain.Answer[bool]{Value: req.Force, Origin: originOf(req.Force)}}
	if !risky {
		questions.Force.Skip = domain.SkipSafe
	}

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, the profile, whether the others' jobs are stopped first, whether a job that changes foreign data starts all the same; then parallel, no_probe and detach, which only their flag sets. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	flagOnly := domain.FieldSpec{Skip: domain.SkipFlagOnly}
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktrees.Origin}, Picked: q.Worktrees.Picked, Options: q.Worktrees.Options},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathProfile, Origin: q.Profile.Origin, Skip: q.Profile.Skip}, Value: q.Profile.Value, Options: q.Profiles},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathExclusive, Origin: q.Exclusive.Origin, Skip: q.Exclusive.Skip}, Value: q.Exclusive.Value, NoFirst: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathForce, Origin: q.Force.Origin, Skip: q.Force.Skip}, Value: q.Force.Value, NoFirst: true},
		domain.ConfirmField{FieldSpec: spec(flagOnly, PathParallel), Value: q.Request.Parallel, NoFirst: true},
		domain.ConfirmField{FieldSpec: spec(flagOnly, PathNoProbe), Value: q.Request.NoProbe, NoFirst: true},
		domain.ConfirmField{FieldSpec: spec(flagOnly, PathDetach), Value: q.Request.Detach, NoFirst: true},
	}}
}

func spec(s domain.FieldSpec, path string) domain.FieldSpec {
	s.Path = path
	return s
}

func originOf(given bool) domain.Origin {
	if given {
		return domain.OriginRequest
	}
	return domain.OriginDefault
}

// picked are the worktrees the request names, else the one the caller stands in.
func (f Facts) picked(req Request) ([]string, domain.Origin) {
	if len(req.Worktrees) > 0 {
		return req.Worktrees, domain.OriginRequest
	}
	if standing := f.standing(); standing != "" {
		return []string{standing}, domain.OriginDefault
	}

	return nil, domain.OriginDefault
}

// profile is the question of the profile: the request's, else the one run.toml marks default, else its only one; skipped when run.toml declares one or none.
func (f Facts) profile(req Request) domain.Choice[string] {
	choice := domain.Choice[string]{Answer: domain.Answer[string]{Origin: domain.OriginDefault}}
	if len(f.Run.Profiles) <= 1 {
		choice.Skip = domain.SkipOneProfile
	}
	if req.Profile != "" {
		choice.Answer = domain.Answer[string]{Value: req.Profile, Origin: domain.OriginRequest}
		return choice
	}
	if profile, ok := f.defaultProfile(); ok {
		choice.Value = profile.Name
		if profile.Default {
			choice.Origin = domain.OriginConfig
		}
	}

	return choice
}

// profileOptions are the profiles run.toml declares, in its order, each with its jobs and whether it is the default.
func (f Facts) profileOptions() []domain.Option {
	options := make([]domain.Option, 0, len(f.Run.Profiles))
	for _, profile := range f.Run.Profiles {
		options = append(options, domain.Option{Value: profile.Name, Default: profile.Default, Jobs: profile.Jobs})
	}
	return options
}

// The codes of the problems of a request: a profile run.toml does not declare, a worktree whose directory is gone or whose record does not read, and --exclusive beside several worktrees or beside --parallel.
const (
	CodeWorktreeGone   = Name + ".worktree_gone"
	CodeMetaUnreadable = Name + domain.MetaUnreadable
	CodeExclusiveMany  = Name + ".exclusive_several"
	CodeBothWays       = Name + ".exclusive_and_parallel"
)

// profileProblems are the problems of the profile asked: one run.toml does not declare, or none when it declares several and none is the default.
func (f Facts) profileProblems(req Request) []domain.FieldError {
	if req.Profile != "" {
		if !slices.ContainsFunc(f.Run.Profiles, func(p project.Profile) bool { return p.Name == req.Profile }) {
			return []domain.FieldError{{Path: PathProfile, Code: domain.CodeNotFound, Template: "{value} is no profile of run.toml", Params: domain.Params{"value": req.Profile}}}
		}
		return nil
	}
	if _, ok := f.defaultProfile(); !ok && len(f.Run.Profiles) > 0 {
		return []domain.FieldError{domain.Required(PathProfile)}
	}
	return nil
}

// exclusion is the question of whether the other worktrees' jobs are stopped first, and its problems.
type exclusion struct {
	domain.Confirm
	problems []domain.FieldError
}

// exclusive settles whether the other worktrees' jobs are stopped first: --exclusive or --parallel, else run.toml's concurrency, else asked when another worktree runs jobs, keeping them by default. run.toml's exclusive cannot stop all but one of several worktrees: it is set aside for them, with a warning the plan gives.
func (f Facts) exclusive(req Request, targets []Target) exclusion {
	switch {
	case req.Parallel && req.Exclusive != nil && *req.Exclusive:
		return exclusion{Confirm: given(true), problems: []domain.FieldError{{Path: PathExclusive, Code: CodeBothWays, Template: "{path} stops the others' jobs, which --parallel keeps", With: []string{PathParallel}}}}
	case req.Parallel:
		return exclusion{Confirm: given(false)}
	case req.Exclusive != nil && *req.Exclusive && len(targets) > 1:
		return exclusion{Confirm: given(true), problems: []domain.FieldError{{Path: PathExclusive, Code: CodeExclusiveMany, Template: "{path} stops all but one worktree: it cannot start several"}}}
	case req.Exclusive != nil:
		return exclusion{Confirm: given(*req.Exclusive)}
	}

	question := domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}}
	switch {
	case len(f.others(targets)) == 0:
		question.Skip = domain.SkipAlone
	case f.Run.Concurrency != "":
		question.Value, question.Origin, question.Skip = f.Run.Concurrency == project.Exclusive && len(targets) <= 1, domain.OriginConfig, domain.SkipConfigured
	}
	return exclusion{Confirm: question}
}

func given(yes bool) domain.Confirm {
	return domain.Confirm{Answer: domain.Answer[bool]{Value: yes, Origin: domain.OriginRequest}}
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, the worktrees defaulting to the caller's and the profile to run.toml's default; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	req.Worktrees, req.Profile = questions.Worktrees.Picked, questions.Profile.Value
	return req, nil
}

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}
