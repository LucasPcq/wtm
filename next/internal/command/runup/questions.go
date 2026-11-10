package runup

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of run up: the worktrees to start a profile in, each by its branch or, on no branch, by its path (none: the one the caller stands in); the Profile of run.toml to start (none: its default); whether the other worktrees' jobs are stopped first (Exclusive, nil leaving it to run.toml, else to the question), or kept (Parallel, which says no to Exclusive); whether a job that changes data the worktree does not own starts all the same (Force); whether the ports the jobs declare go unchecked (NoProbe); whether the command line returns once they are started rather than show them (Detach); and the answers to remember (Remember: PathExclusive answered no is run.toml's concurrency = "parallel" from then on). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	Profile   string        `json:"profile"`
	Exclusive *bool         `json:"exclusive"`
	Parallel  bool          `json:"parallel"`
	Force     bool          `json:"force"`
	NoProbe   bool          `json:"no_probe"`
	Detach    bool          `json:"detach"`
	Remember  []string      `json:"remember"`
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
	PathExclusive = runjobs.PathExclusive
	PathParallel  = runjobs.PathParallel
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
	case domain.PathRemember:
		next.Remember = values
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
	case domain.PathRemember:
		return r.Remember
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
	targets, problems := f.Entries(Name, picked)
	questions := Questions{
		Worktrees: Worktrees{Picked: picked, Origin: origin, Options: f.WorktreeOptions()},
		Profile:   f.profile(req),
		Profiles:  f.profileOptions(targets),
		Request:   req,
		Errors:    problems,
	}
	if len(picked) == 0 {
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	questions.Errors = append(questions.Errors, f.profileProblems(req)...)

	exclusive := f.exclusive(req, targets)
	questions.Exclusive = exclusive.Confirm
	questions.Errors = append(questions.Errors, exclusive.Problems...)
	questions.Errors = append(questions.Errors, rememberProblems(req, exclusive.Value)...)

	risky := f.Risky(targets, f.jobsOf(questions.Profile.Value))
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
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathExclusive, Origin: q.Exclusive.Origin, Skip: q.Exclusive.Skip}, Value: q.Exclusive.Value, NoFirst: true, Rememberable: true, Remember: slices.Contains(q.Request.Remember, PathExclusive)},
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
	if standing := f.Standing(); standing != "" {
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

// profileOptions are the profiles run.toml declares, in its order, each with its jobs, whether it is the default and which of the targets run it now.
func (f Facts) profileOptions(targets []runjobs.Target) []domain.Option {
	options := make([]domain.Option, 0, len(f.Run.Profiles))
	for _, profile := range f.Run.Profiles {
		option := domain.Option{Value: profile.Name, Default: profile.Default, Jobs: profile.Jobs}
		for _, t := range targets {
			if run.ProfileIn(f.Jobs, t.Path) == profile.Name {
				option.RunningIn = append(option.RunningIn, t.Worktree)
			}
		}
		options = append(options, option)
	}
	return options
}

// The codes of the problems of a request: a profile run.toml does not declare, a worktree whose directory is gone or whose record does not read, and --exclusive beside several worktrees or beside --parallel.
const (
	CodeWorktreeGone   = Name + runjobs.WorktreeGone
	CodeMetaUnreadable = Name + domain.MetaUnreadable
	CodeExclusiveMany  = Name + runjobs.ExclusiveSeveral
	CodeBothWays       = Name + runjobs.ExclusiveAndParallel
)

// CodeRememberExclusive refuses to remember stopping the other worktrees' jobs: only keeping them is remembered, in run.toml.
const CodeRememberExclusive = Name + ".remember_exclusive"

// rememberProblems are the problems of the answers to remember: one that is no rememberable field, and the others' jobs stopped first, which is never remembered.
func rememberProblems(req Request, exclusive bool) []domain.FieldError {
	var problems []domain.FieldError
	for _, path := range req.Remember {
		switch {
		case path != PathExclusive:
			problems = append(problems, domain.OneOf(domain.OneOfParams{Path: domain.PathRemember, Value: path, Accepted: []string{PathExclusive}})...)
		case exclusive:
			problems = append(problems, domain.FieldError{Path: PathExclusive, Code: CodeRememberExclusive, Template: "{path} is never remembered: only keeping the other worktrees' jobs is", With: []string{domain.PathRemember}})
		}
	}
	return problems
}

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

// exclusive settles whether the other worktrees' jobs are stopped first (runjobs.Facts.Exclusive): --exclusive or --parallel, else run.toml's concurrency, else asked when another worktree runs jobs.
func (f Facts) exclusive(req Request, targets []runjobs.Target) runjobs.Exclusion {
	return f.Exclusive(runjobs.ExclusiveParams{Command: Name, Exclusive: req.Exclusive, Parallel: req.Parallel, Targets: targets})
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
