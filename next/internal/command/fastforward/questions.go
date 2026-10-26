package fastforward

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of fast-forward: the worktrees it names, each by its branch or its path, or every worktree; and whether a worktree with uncommitted changes is fast-forwarded all the same. The worktrees it names are the user's choice, every one is the command's: a named worktree it cannot move safely refuses the request, one it selected is left as it is. Caller is where the caller stands, from which a path is read; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	All       bool          `json:"all"`
	Force     bool          `json:"force"`
	Caller    domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of fast-forward's own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathAll   = "all"
	PathForce = "force"
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
	case PathForce:
		next.Force, err = domain.Yes(path, values)
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
	case PathForce:
		return domain.SaidYes(r.Force)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees to fast-forward and those it may, whether every one is, whether to force the worktrees Unsafe names, and the request's problems as field errors.
type Questions struct {
	Worktrees Worktrees
	All       bool
	Force     domain.Confirm
	Unsafe    []Advance
	Errors    []domain.FieldError
}

// Worktrees is the question of the worktrees to fast-forward: those given, else those behind origin as last fetched with no uncommitted changes, its default pick; and every worktree on a branch, the main one first, each with where it stands against origin as last fetched and its uncommitted changes.
type Worktrees struct {
	Picked  domain.Answer[[]string]
	Options []domain.Option
}

// NewQuestions builds the questions for the request on the facts, as last fetched: the worktrees given, else those every worktree asked would move picked by default; the worktrees named behind origin with uncommitted changes, and whether to force them, asked only when there are some: every worktree asked leaves them as they are instead.
func NewQuestions(f Facts, req Request) Questions {
	advances, problems := f.entries(req)
	questions := Questions{
		Worktrees: Worktrees{Picked: domain.Answer[[]string]{Value: req.Worktrees, Origin: domain.OriginRequest}, Options: f.worktreeOptions(req.Caller.Cwd)},
		All:       req.All,
		Errors:    problems,
	}
	if !req.All {
		questions.Unsafe = unsafe(advances)
	}
	if len(req.Worktrees) == 0 && !req.All {
		questions.Worktrees.Picked = domain.Answer[[]string]{Value: f.behind(), Origin: domain.OriginDefault}
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	questions.Force = confirm(req.Force, len(questions.Unsafe) == 0)

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, unless every one is fast-forwarded, then whether every one is, a switch only its flag sets, then whether to force the unsafe ones. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	worktrees := domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktrees.Picked.Origin}
	if q.All {
		worktrees.Skip = domain.SkipAll
	}

	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: worktrees, Picked: q.Worktrees.Picked.Value, Options: q.Worktrees.Options},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathAll, Skip: domain.SkipFlagOnly}, Value: q.All, NoFirst: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathForce, Origin: q.Force.Origin, Skip: q.Force.Skip}, Value: q.Force.Value, NoFirst: true},
	}}
}

// worktreeOptions are the worktrees fast-forward may move: each on a branch, the main one first, tagged as every picker tags them from where the caller stands at cwd, with where it stands against origin as last fetched and its uncommitted changes.
func (f Facts) worktreeOptions(cwd string) []domain.Option {
	options := domain.WorktreeOptions(f.onBranch(), domain.WorktreeQuery{Cwd: cwd})
	for i, option := range options {
		options[i].Origin, options[i].Changes = f.OriginOf(option.Value), f.Checks[option.Value].Changes
	}

	return options
}

// confirm is the force switch: given, it stands as the request's; else no by default, not asked when no worktree asked is unsafe.
func confirm(given, safe bool) domain.Confirm {
	switch {
	case given:
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	case safe:
		return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: domain.SkipSafe}
	}

	return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}}
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

// CodeAllWithWorktree refuses every worktree asked with some named; CodeDetached refuses a worktree on no branch, which has nothing to fast-forward.
const (
	CodeAllWithWorktree = Name + ".all_with_worktree"
	CodeDetached        = Name + ".detached"
)

// entries are the worktrees req asks, as advances in the order asked, every one on a branch the main first with All, and the problem of each entry it cannot take: blank, given twice, a worktree on no branch, a name that is no worktree, a name with All.
func (f Facts) entries(req Request) (advances []Advance, problems []domain.FieldError) {
	if req.All {
		if len(req.Worktrees) > 0 {
			problems = append(problems, domain.FieldError{Path: PathAll, Code: CodeAllWithWorktree, Template: "{path} fast-forwards every worktree: name none with it"})
		}
		for index, w := range f.onBranch() {
			advances = append(advances, f.advance(w, index))
		}
		return advances, problems
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
			problems = append(problems, domain.FieldError{Path: path, Code: CodeDetached, Template: "{value} is on no branch: nothing to fast-forward", Params: domain.Params{"value": name}})
		case slices.ContainsFunc(advances, func(a Advance) bool { return a.Worktree == w.Name() }):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} names a worktree given already", Params: domain.Params{"value": name}})
		default:
			advances = append(advances, f.advance(w, index))
		}
	}

	return advances, problems
}

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}
