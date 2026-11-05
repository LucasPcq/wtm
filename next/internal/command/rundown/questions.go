package rundown

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of run down: the worktrees to stop the jobs of, each by its branch, by its path on no branch, or by the path of a worktree removed whose jobs still run (none: the one the caller stands in), or All of this repository's; the Profile of run.toml whose jobs alone stop (none: every job). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	Profile   string        `json:"profile"`
	All       bool          `json:"all"`
	Caller    domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of run down's own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathProfile = "profile"
	PathAll     = "all"
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
	case PathAll:
		next.All, err = domain.Yes(path, values)
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
	case PathAll:
		return domain.SaidYes(r.All)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees and those it may stop the jobs of, each with what it runs; the profiles run.toml declares; and the request's problems as field errors.
type Questions struct {
	Picked   []string
	Origin   domain.Origin
	Options  []domain.Option
	Profiles []domain.Option
	Request  Request
	Errors   []domain.FieldError
}

// NewQuestions builds the questions for the request on the facts.
func NewQuestions(f Facts, req Request) Questions {
	picked, origin := f.picked(req)
	_, problems := f.entries(picked)
	questions := Questions{Picked: picked, Origin: origin, Options: f.worktreeOptions(), Profiles: f.profileOptions(), Request: req, Errors: problems}
	if len(picked) == 0 && !req.All {
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	if req.All && (len(req.Worktrees) > 0 || req.Profile != "") {
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathAll, Code: CodeAllNarrowed, Template: "{path} stops every worktree's jobs: name no worktree nor profile with it", With: []string{domain.PathWorktree, PathProfile}})
	}
	switch {
	case req.Profile != "" && f.RunErr != nil:
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathProfile, Code: domain.CodeNotFound, Template: "{value} cannot be read: run.toml does not load ({reason})", Params: domain.Params{"value": req.Profile, "reason": f.RunErr.Error()}})
	case req.Profile != "" && !slices.ContainsFunc(f.Run.Profiles, func(p project.Profile) bool { return p.Name == req.Profile }):
		questions.Errors = append(questions.Errors, domain.FieldError{Path: PathProfile, Code: domain.CodeNotFound, Template: "{value} is no profile of run.toml", Params: domain.Params{"value": req.Profile}})
	}

	return questions
}

// CodeAllNarrowed is the problem of --all beside a worktree or a profile.
const CodeAllNarrowed = Name + ".all_narrowed"

// Form is the questions as the schema a surface renders: the worktrees, unless all of them; then the profile and all, which only their flag sets. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	worktrees := domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Origin}, Picked: q.Picked, Options: q.Options}
	if q.Request.All {
		worktrees.Skip = domain.SkipAll
	}
	flagOnly := domain.FieldSpec{Skip: domain.SkipFlagOnly}

	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		worktrees,
		domain.SelectField{FieldSpec: spec(flagOnly, PathProfile), Value: q.Request.Profile, Options: q.Profiles},
		domain.ConfirmField{FieldSpec: spec(flagOnly, PathAll), Value: q.Request.All, NoFirst: true},
	}}
}

func spec(s domain.FieldSpec, path string) domain.FieldSpec {
	s.Path = path
	return s
}

// picked are the worktrees the request names, else the one the caller stands in; none with all.
func (f Facts) picked(req Request) ([]string, domain.Origin) {
	switch {
	case len(req.Worktrees) > 0:
		return req.Worktrees, domain.OriginRequest
	case req.All:
		return nil, domain.OriginRequest
	}
	if standing := f.standing(); standing != "" {
		return []string{standing}, domain.OriginDefault
	}

	return nil, domain.OriginDefault
}

// profileOptions are the profiles run.toml declares, in its order, each with its jobs.
func (f Facts) profileOptions() []domain.Option {
	options := make([]domain.Option, 0, len(f.Run.Profiles))
	for _, profile := range f.Run.Profiles {
		options = append(options, domain.Option{Value: profile.Name, Default: profile.Default, Jobs: profile.Jobs})
	}
	return options
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

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}
