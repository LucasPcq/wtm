package runstart

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

// A Request is what the caller asks of run start: the worktree to start one job in, by its branch or, on no branch, by its path (none: the one the caller stands in); the Job of run.toml to start; whether a job up there is stopped and started again (Restart) rather than left as it is; whether the other worktrees' jobs are stopped first (Exclusive, nil leaving it to run.toml, else to the question), or kept (Parallel, which says no to Exclusive); whether a job that changes data the worktree does not own starts all the same (Force); whether the ports it declares go unchecked (NoProbe); and whether the command line returns once it is started rather than show it (Detach). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktree  string        `json:"worktree"`
	Job       string        `json:"job"`
	Restart   bool          `json:"restart"`
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

// The paths of run start's own fields, by which each surface words them; the worktree's is domain.PathWorktree.
const (
	PathJob       = "job"
	PathRestart   = "restart"
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
		next.Worktree = strings.Join(values, "")
	case PathJob:
		next.Job = strings.Join(values, "")
	case PathRestart:
		next.Restart, err = domain.Yes(path, values)
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
		return nonEmpty(r.Worktree)
	case PathJob:
		return nonEmpty(r.Job)
	case PathRestart:
		return domain.SaidYes(r.Restart)
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

func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktree and those it may start in, the job and those run.toml declares, whether the others' jobs are stopped first, whether a job that changes foreign data starts all the same, and the request's problems as field errors.
type Questions struct {
	Worktree  domain.Choice[string]
	Options   []domain.Option
	Jobs      []domain.Option
	Exclusive domain.Confirm
	Force     domain.Confirm
	Request   Request
	Errors    []domain.FieldError
}

// The codes of the problems of a request: a worktree whose directory is gone or whose record does not read, --exclusive beside --parallel, and --restart of a shared service up elsewhere that the worktree does not hold.
const (
	CodeWorktreeGone   = Name + runjobs.WorktreeGone
	CodeMetaUnreadable = Name + domain.MetaUnreadable
	CodeBothWays       = Name + runjobs.ExclusiveAndParallel
	CodeRestartShared  = Name + ".restart_shared"
)

// NewQuestions builds the questions for the request on the facts.
func NewQuestions(f Facts, req Request) Questions {
	worktree := f.worktree(req)
	var picked []string
	if worktree.Value != "" {
		picked = []string{worktree.Value}
	}
	targets, problems := f.Entries(Name, picked)
	questions := Questions{Worktree: worktree, Options: f.WorktreeOptions(), Jobs: f.jobOptions(targets), Request: req, Errors: problems}
	if worktree.Value == "" {
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	job, declared := f.Declared(req.Job)
	switch {
	case req.Job == "":
		questions.Errors = append(questions.Errors, domain.Required(PathJob))
	case !declared:
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathJob, Code: domain.CodeNotFound, Template: "{value} is no job of run.toml", Params: domain.Params{"value": req.Job}})
	case req.Restart && job.Scope == project.Shared && f.upAnywhere(req.Job) && !f.holds(targets, req.Job):
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathRestart, Code: CodeRestartShared, Template: "{job} is a shared service {worktree} does not hold: only a worktree holding it restarts it", Params: domain.Params{"job": req.Job, "worktree": worktree.Value}})
	}

	exclusive := f.exclusive(req, targets, job)
	questions.Exclusive = exclusive.Confirm
	questions.Errors = append(questions.Errors, exclusive.Problems...)

	questions.Force = domain.Confirm{Answer: domain.Answer[bool]{Value: req.Force, Origin: originOf(req.Force)}}
	if !declared || !f.Risky(targets, []project.Job{job}) {
		questions.Force.Skip = domain.SkipSafe
	}
	return questions
}

// Form is the questions as the schema a surface renders: the worktree, the job, whether the others' jobs are stopped first, whether a job that changes foreign data starts all the same; then restart, parallel, no_probe and detach, which only their flag sets. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	flagOnly := domain.FieldSpec{Skip: domain.SkipFlagOnly}
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktree.Origin}, Value: q.Worktree.Value, Options: q.Options},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathJob}, Value: q.Request.Job, Options: q.Jobs},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathExclusive, Origin: q.Exclusive.Origin, Skip: q.Exclusive.Skip}, Value: q.Exclusive.Value, NoFirst: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathForce, Origin: q.Force.Origin, Skip: q.Force.Skip}, Value: q.Force.Value, NoFirst: true},
		domain.ConfirmField{FieldSpec: spec(flagOnly, PathRestart), Value: q.Request.Restart, NoFirst: true},
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

// worktree is the question of the worktree: the request's, else the one the caller stands in.
func (f Facts) worktree(req Request) domain.Choice[string] {
	if req.Worktree != "" {
		return domain.Choice[string]{Answer: domain.Answer[string]{Value: req.Worktree, Origin: domain.OriginRequest}}
	}
	return domain.Choice[string]{Answer: domain.Answer[string]{Value: f.Standing(), Origin: domain.OriginDefault}}
}

// jobOptions are the jobs of run.toml, in its order, each with its kind and, when it is up in the worktree of targets, that worktree.
func (f Facts) jobOptions(targets []runjobs.Target) []domain.Option {
	options := make([]domain.Option, 0, len(f.Run.Jobs))
	for _, job := range f.Run.Jobs {
		option := domain.Option{Value: job.Name, Kind: job.Kind}
		for _, t := range targets {
			if slices.Contains(run.UpIn(f.Jobs, t.Path), job.Name) {
				option.RunningIn = append(option.RunningIn, t.Worktree)
			}
		}
		options = append(options, option)
	}
	return options
}

// exclusive settles whether the other worktrees' jobs are stopped first (runjobs.Facts.Exclusive): --exclusive or --parallel, else run.toml's concurrency, else asked when another worktree runs jobs; never for a task, which binds no port and is gone once done.
func (f Facts) exclusive(req Request, targets []runjobs.Target, job project.Job) runjobs.Exclusion {
	settled := f.Exclusive(runjobs.ExclusiveParams{Command: Name, Exclusive: req.Exclusive, Parallel: req.Parallel, Targets: targets})
	if job.Kind == project.Task && req.Exclusive == nil {
		settled.Value, settled.Skip, settled.Origin = false, domain.SkipTask, domain.OriginDefault
	}
	return settled
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, the worktree defaulting to the caller's; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	req.Worktree = questions.Worktree.Value
	return req, nil
}
