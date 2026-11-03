package runlogs

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of run logs: the worktrees whose jobs it reads, each by its branch or, on no branch, by its path (none: the one the caller stands in), the Job of run.toml to read alone (none: every job that ran there), and whether its lines are followed as the jobs print them (Follow), past the last ones. Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	Job       string        `json:"job"`
	Follow    bool          `json:"follow"`
	Caller    domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of run logs' own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathJob    = "job"
	PathFollow = "follow"
)

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktrees = values
	case PathJob:
		next.Job = strings.Join(values, "")
	case PathFollow:
		next.Follow, err = domain.Yes(path, values)
	default:
		return r, domain.Unknown(path)
	}

	if err != nil {
		return r, err
	}
	return next, nil
}

// Values is the request's answer to the field at path, nil when it leaves the field to its default.
func (r Request) Values(path string) []string {
	switch path {
	case domain.PathWorktree:
		return r.Worktrees
	case PathJob:
		return domain.Spelled(r.Job)
	case PathFollow:
		return domain.SaidYes(r.Follow)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees and those it may read, the job and those run.toml declares, and the request's problems as field errors.
type Questions struct {
	Worktrees Worktrees
	Job       string
	Jobs      []domain.Option
	Follow    bool
	Errors    []domain.FieldError
}

// Worktrees is the question of the worktrees to read: those picked, the request's or by default the one the caller stands in, and every one that stands.
type Worktrees struct {
	Picked  []string
	Origin  domain.Origin
	Options []domain.Option
}

// The codes of the problems of a request: a worktree whose directory is gone, a job neither run.toml declares nor the daemon lists in the worktrees asked.
const (
	CodeWorktreeGone = Name + ".worktree_gone"
	CodeJobUnknown   = Name + ".job_unknown"
)

// NewQuestions builds the questions for the request on the facts.
func NewQuestions(f Facts, req Request) Questions {
	picked, origin := f.picked(req)
	targets, problems := f.targets(picked)
	standing := f.standing()
	questions := Questions{
		Worktrees: Worktrees{Picked: picked, Origin: origin, Options: domain.RunningIn(domain.WorktreeOptions(standing, domain.WorktreeQuery{Cwd: f.Cwd}), standing, f.Jobs)},
		Job:       req.Job,
		Jobs:      jobOptions(f.Run.Jobs),
		Follow:    req.Follow,
		Errors:    problems,
	}
	if len(picked) == 0 {
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	if req.Job != "" && !f.knows(req.Job, targets) {
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathJob, Code: CodeJobUnknown, Template: "{value} is no job of run.toml, nor one the run daemon runs in these worktrees", Params: domain.Params{"value": req.Job}})
	}

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, the job, then whether to follow, which only its flag sets. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktrees.Origin}, Picked: q.Worktrees.Picked, Options: q.Worktrees.Options},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathJob, Origin: q.jobOrigin()}, Value: q.Job, Options: q.Jobs},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathFollow, Skip: domain.SkipFlagOnly}, Value: q.Follow, NoFirst: true},
	}}
}

// knows reports whether name is a job run.toml declares, or one the daemon lists in one of targets (run.toml renamed or dropped it since it started).
func (f Facts) knows(name string, targets []Target) bool {
	if slices.ContainsFunc(f.Run.Jobs, func(job project.Job) bool { return job.Name == name }) {
		return true
	}
	return slices.ContainsFunc(f.Jobs, func(job run.Job) bool {
		return job.Name == name && slices.ContainsFunc(targets, func(t Target) bool { return t.Path == job.WorkDir })
	})
}

func jobOptions(jobs []project.Job) []domain.Option {
	options := make([]domain.Option, 0, len(jobs))
	for _, job := range jobs {
		options = append(options, domain.Option{Value: job.Name})
	}
	return options
}

// jobOrigin is where the job asked comes from: the request, or none when it asks every job.
func (q Questions) jobOrigin() domain.Origin {
	if q.Job == "" {
		return ""
	}
	return domain.OriginRequest
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, the worktrees defaulting to the caller's, and returns the worktrees it reads; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, []Target, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, nil, domain.Invalid(questions.Errors)
	}

	req.Worktrees = questions.Worktrees.Picked
	targets, _ := f.targets(req.Worktrees)
	return req, targets, nil
}

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}
