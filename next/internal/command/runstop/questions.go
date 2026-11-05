package runstop

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of run stop: the worktrees to stop one job in, each by its branch, by its path on no branch, or by the path of a worktree removed whose jobs still run (none: the one the caller stands in), and the Job to stop there. Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	Job       string        `json:"job"`
	Caller    domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// PathJob is the path of run stop's own field, the job, by which each surface words it; the worktrees' is domain.PathWorktree.
const PathJob = "job"

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	switch path {
	case domain.PathWorktree:
		r.Worktrees = values
	case PathJob:
		r.Job = strings.Join(values, "")
	default:
		return r, domain.Unknown(path)
	}
	return r, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves the field to its default.
func (r Request) Values(path string) []string {
	switch path {
	case domain.PathWorktree:
		return r.Worktrees
	case PathJob:
		if r.Job == "" {
			return nil
		}
		return []string{r.Job}
	}
	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees and those it may stop a job in, each with what it runs; the jobs it may stop, each with its kind and the worktrees picked it is up in; and the request's problems as field errors.
type Questions struct {
	Picked  []string
	Origin  domain.Origin
	Options []domain.Option
	Jobs    []domain.Option
	Request Request
	Errors  []domain.FieldError
}

// NewQuestions builds the questions for the request on the facts.
func NewQuestions(f Facts, req Request) Questions {
	picked, origin := f.picked(req)
	targets, problems := f.Entries(picked)
	questions := Questions{Picked: picked, Origin: origin, Options: f.WorktreeOptions(), Jobs: f.jobOptions(targets), Request: req, Errors: problems}
	switch {
	case len(picked) == 0:
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	case req.Job == "":
		questions.Errors = append(questions.Errors, domain.Required(PathJob))
	case !slices.ContainsFunc(questions.Jobs, func(o domain.Option) bool { return o.Value == req.Job }):
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathJob, Code: domain.CodeNotFound, Template: "{value} is neither a job of run.toml nor one the run daemon runs in the worktrees picked", Params: domain.Params{"value": req.Job}})
	}
	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, then the job.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Origin}, Picked: q.Picked, Options: q.Options},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathJob}, Value: q.Request.Job, Options: q.Jobs},
	}}
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

// jobOptions are the jobs a stop of targets may reach: run.toml's, in its order, then those the run daemon runs in targets that run.toml does not declare, by name; each with its kind and the worktrees of targets it is up in.
func (f Facts) jobOptions(targets []Target) []domain.Option {
	upIn := func(name string) []string {
		var worktrees []string
		for _, t := range targets {
			if slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.Up && job.WorkDir == t.Path && job.Name == name }) {
				worktrees = append(worktrees, t.Worktree)
			}
		}
		return worktrees
	}

	options := make([]domain.Option, 0, len(f.Run.Jobs))
	for _, job := range f.Run.Jobs {
		options = append(options, domain.Option{Value: job.Name, Kind: job.Kind, RunningIn: upIn(job.Name)})
	}
	var undeclared []domain.Option
	for _, job := range f.Jobs {
		declared := slices.ContainsFunc(f.Run.Jobs, func(j project.Job) bool { return j.Name == job.Name })
		listed := slices.ContainsFunc(undeclared, func(o domain.Option) bool { return o.Value == job.Name })
		if !declared && !listed && slices.ContainsFunc(targets, func(t Target) bool { return t.Path == job.WorkDir }) {
			undeclared = append(undeclared, domain.Option{Value: job.Name, Kind: job.Kind, RunningIn: upIn(job.Name)})
		}
	}
	slices.SortFunc(undeclared, func(a, b domain.Option) int { return strings.Compare(a.Value, b.Value) })
	return append(options, undeclared...)
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, the worktrees defaulting to the caller's; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	req.Worktrees = questions.Picked
	return req, nil
}
