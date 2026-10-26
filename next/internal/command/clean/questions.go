package clean

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of clean: the worktrees to remove, each by its branch or, on no branch, by its path; whether the children they leave behind move onto the nearest worktree that stays; and whether a worktree unsafe to remove is removed all the same. Caller is where the caller stands, which a removal of its own worktree steps out of; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees        []string      `json:"worktree"`
	ReparentChildren bool          `json:"reparent_children"`
	Force            bool          `json:"force"`
	Caller           domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of clean's own fields, by which each surface words them; the worktrees' is domain.PathWorktree, the path every follow-up that removes a worktree answers.
const (
	PathReparentChildren = "reparent_children"
	PathForce            = "force"
)

// Answer is the request with the field at path answered by values. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktrees = values
	case PathReparentChildren:
		next.ReparentChildren, err = domain.Yes(path, values)
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
	case PathReparentChildren:
		return domain.SaidYes(r.ReparentChildren)
	case PathForce:
		return domain.SaidYes(r.Force)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees to remove and those it may remove, whether to reparent the children left behind, named in Orphans, whether to force the removal of the worktrees Unsafe names, and the request's problems as field errors.
type Questions struct {
	Worktrees Worktrees
	Reparent  domain.Confirm
	Force     domain.Confirm
	Orphans   []teardown.Move
	Unsafe    []teardown.Removal
	Errors    []domain.FieldError
}

// Worktrees is the question of the worktrees to remove: those given, and every worktree but the main one and the base branch's, by its branch or, on no branch, its path.
type Worktrees struct {
	Given   []string
	Options []domain.Option
}

// NewQuestions builds the questions for the request on the facts: the worktrees given; the children their removal leaves behind, and whether to reparent them, asked only when there are some; the worktrees unsafe to remove, and whether to force them, asked only when there are some. A switch given stands as given.
func NewQuestions(f Facts, req Request) Questions {
	removals, _, problems := f.entries(req.Worktrees)
	questions := Questions{
		Worktrees: Worktrees{Given: req.Worktrees, Options: f.worktreeOptions()},
		Orphans:   f.Moves(removals),
		Unsafe:    teardown.Blocked(removals),
		Errors:    problems,
	}
	questions.Reparent = confirm(req.ReparentChildren, len(questions.Orphans) == 0, domain.SkipNoOrphan)
	questions.Force = confirm(req.Force, len(questions.Unsafe) == 0, domain.SkipSafe)
	if len(req.Worktrees) == 0 {
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, then whether to reparent the children left behind, then whether to force the unsafe ones. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree}, Picked: q.Worktrees.Given, Options: q.Worktrees.Options},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathReparentChildren, Origin: q.Reparent.Origin, Skip: q.Reparent.Skip}, Value: q.Reparent.Value, NoFirst: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathForce, Origin: q.Force.Origin, Skip: q.Force.Skip}, Value: q.Force.Value, NoFirst: true},
	}}
}

// worktreeOptions are the worktrees clean may remove: each but the main one and the base branch's, by its branch or, on no branch, its path, in git's order.
func (f Facts) worktreeOptions() []domain.Option {
	var options []domain.Option
	for _, w := range f.Worktrees {
		if !w.Main && w.Branch() != f.Config.Worktrees.BaseBranch {
			options = append(options, domain.Option{Value: w.Name(), Detached: w.Branch() == ""})
		}
	}

	return options
}

// confirm is a switch: given, it stands as the request's; else no by default, not asked when moot says nothing would be asked of it.
func confirm(given, moot bool, skip domain.Skip) domain.Confirm {
	switch {
	case given:
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	case moot:
		return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: skip}
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
