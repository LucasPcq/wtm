package exec

import (
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of exec: the worktrees to run in, each by its branch or, on no branch, by its path, or All of them; the shell line to run in each; how many run at once (Jobs, a whole number, empty or 0 for one per CPU); and whether each one's whole output is kept (Print). Caller is where the caller stands, whose worktree the form picks first; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	All       bool          `json:"all"`
	Command   string        `json:"command"`
	Jobs      string        `json:"jobs"`
	Print     bool          `json:"print"`
	Caller    domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of exec's own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathAll     = "all"
	PathCommand = "command"
	PathJobs    = "jobs"
	PathPrint   = "print"
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
	case PathCommand:
		next.Command = strings.Join(values, " ")
	case PathJobs:
		next.Jobs = strings.Join(values, "")
	case PathPrint:
		next.Print, err = domain.Yes(path, values)
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
	case PathCommand:
		return given(r.Command)
	case PathJobs:
		return given(r.Jobs)
	case PathPrint:
		return domain.SaidYes(r.Print)
	}

	return nil
}

func given(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees it may run in, the caller's picked first when none is given, and the request's problems as field errors.
type Questions struct {
	Request Request
	Picked  []string
	Options []domain.Option
	Errors  []domain.FieldError
}

// NewQuestions builds the questions for the request on the facts.
func NewQuestions(f Facts, req Request) Questions {
	_, problems := f.entries(req.Worktrees)
	questions := Questions{Request: req, Picked: req.Worktrees, Options: f.options(), Errors: problems}
	if len(req.Worktrees) == 0 && !req.All {
		questions.Picked = f.standing()
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	if req.All && len(req.Worktrees) > 0 {
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathAll, Code: CodeAllWithWorktrees, Template: "{path} runs in every worktree: name none with it", With: []string{domain.PathWorktree}})
	}

	switch {
	case strings.TrimSpace(req.Command) == "":
		questions.Errors = append(questions.Errors, domain.Required(PathCommand))
	case f.Syntax != nil:
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathCommand, Code: CodeCommandInvalid, Template: "{path}: {reason}", Params: domain.Params{"reason": f.Syntax.Error()}})
	}
	if _, ok := jobsOf(req.Jobs); !ok {
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathJobs, Code: CodeJobsInvalid, Template: "{path} takes a whole number, 0 or more, not {value}", Params: domain.Params{"value": req.Jobs}})
	}

	return questions
}

// The codes of the problems of a request: a command the shell refuses, --all beside worktrees named, a concurrency that is no whole number, and a worktree whose directory is gone.
const (
	CodeCommandInvalid   = Name + ".command_invalid"
	CodeAllWithWorktrees = Name + ".all_with_worktrees"
	CodeJobsInvalid      = Name + ".jobs_invalid"
	CodeWorktreeGone     = Name + ".worktree_gone"
)

// Form is the questions as the schema a surface renders: the worktrees, unless all of them; the command; then all, the concurrency and print, which only their flag sets. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	req := q.Request
	worktrees := domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree}, Picked: q.Picked, Options: q.Options}
	if req.All {
		worktrees.Skip = domain.SkipAll
	}
	flagOnly := domain.FieldSpec{Skip: domain.SkipFlagOnly}

	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		worktrees,
		domain.TextField{FieldSpec: domain.FieldSpec{Path: PathCommand}, Value: req.Command},
		domain.ConfirmField{FieldSpec: spec(flagOnly, PathAll), Value: req.All, NoFirst: true},
		domain.TextField{FieldSpec: spec(flagOnly, PathJobs), Value: req.Jobs},
		domain.ConfirmField{FieldSpec: spec(flagOnly, PathPrint), Value: req.Print, NoFirst: true},
	}}
}

func spec(s domain.FieldSpec, path string) domain.FieldSpec {
	s.Path = path
	return s
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

// jobsOf is how many commands jobs runs at once, 0 for one per CPU, and whether it is a whole number, 0 or more; empty is 0.
func jobsOf(jobs string) (int, bool) {
	if jobs == "" {
		return 0, true
	}
	n, err := strconv.Atoi(jobs)
	return n, err == nil && n >= 0
}

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}
