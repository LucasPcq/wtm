package reparent

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of reparent: the worktrees to move, each by its branch, none for the one the caller stands in; and the branch they are rebased onto from now on. Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees []string      `json:"worktree"`
	To        string        `json:"to"`
	Caller    domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// PathTo is the path of the new parent, by which each surface words it; the worktrees' is domain.PathWorktree, the path every follow-up that moves a worktree answers.
const PathTo = "to"

// Answer is the request with the field at path answered by values. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktrees = values
	case PathTo:
		next.To, err = domain.One[string](path, values)
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
	case PathTo:
		return domain.Spelled(r.To)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees to move and those that can be, the new parent and the branches it can be, the move of each worktree given, and the request's problems as field errors.
type Questions struct {
	Worktrees Worktrees
	To        To
	Moves     []Move
	Errors    []domain.FieldError
}

// Worktrees is the question of the worktrees to move: those picked, the request's or by default the one the caller stands in, and every worktree that can be moved, each with its parent.
type Worktrees struct {
	Picked  []string
	Origin  domain.Origin
	Options []domain.Option
}

// To is the question of the new parent: its answer, the branches it can be, the base branch listed first, and those it leaves out, each with the problem it would raise.
type To struct {
	domain.Answer[string]
	Options []domain.Option
	Pinned  string
	Refused []domain.FieldError
}

// NewQuestions builds the questions for the request on the facts: the worktrees picked, the moves they make, and the branches the new parent can be, those that would close a cycle with the worktrees given left out.
func NewQuestions(f Facts, req Request) Questions {
	picked, origin := f.picked(req)
	moves, problems := f.entries(picked)
	questions := Questions{
		Worktrees: Worktrees{Picked: picked, Origin: origin, Options: f.worktreeOptions()},
		To:        f.to(req, branches(moves)),
		Moves:     moves,
		Errors:    problems,
	}
	for i := range questions.Moves {
		questions.Moves[i].To = req.To
	}
	if len(picked) == 0 {
		questions.Errors = append(questions.Errors, domain.Required(domain.PathWorktree))
	}
	if req.To == "" {
		questions.Errors = append(questions.Errors, domain.Required(PathTo))
	}
	if problem := f.problemOf(req.To, branches(moves)); problem != nil {
		questions.Errors = append(questions.Errors, *problem)
	}

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, then the new parent. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktrees.Origin}, Picked: q.Worktrees.Picked, Options: q.Worktrees.Options},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathTo, Origin: q.To.Origin}, Value: q.To.Value, Options: q.To.Options, Pinned: q.To.Pinned, Refreshable: true, Refused: q.To.Refused},
	}}
}

// picked are the worktrees the request names, else the one the caller stands in when it can be moved.
func (f Facts) picked(req Request) ([]string, domain.Origin) {
	if len(req.Worktrees) > 0 {
		return req.Worktrees, domain.OriginRequest
	}
	if current := f.current(); current != "" {
		return []string{current}, domain.OriginDefault
	}

	return nil, domain.OriginDefault
}

// worktreeOptions are the worktrees reparent can move, in git's order, each with the parent it is rebased onto and the caller's tagged current.
func (f Facts) worktreeOptions() []domain.Option {
	current := f.current()
	var options []domain.Option
	for _, w := range f.Worktrees {
		if !f.movable(w) {
			continue
		}
		option := domain.Option{Value: w.Branch(), Parent: f.Records[w.Branch()].SourceBranch}
		if w.Branch() == current {
			option.Tag = domain.TagCurrent
		}
		options = append(options, option)
	}

	return options
}

// to is the question of the new parent for the worktrees moving: every branch a parent can be, the base branch first, but those moving and those under them, which are refused.
func (f Facts) to(req Request, moving []string) To {
	question := To{Pinned: f.Config.Worktrees.BaseBranch, Answer: domain.Answer[string]{Origin: domain.OriginDefault}}
	if req.To != "" {
		question.Answer = domain.Answer[string]{Value: req.To, Origin: domain.OriginRequest}
	}
	for _, option := range domain.SourceOptions(f.Repository) {
		if problem := f.problemOf(option.Value, moving); problem != nil {
			question.Refused = append(question.Refused, *problem)
			continue
		}
		question.Options = append(question.Options, option)
	}

	return question
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, the worktrees defaulting to the caller's; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	req.Worktrees = questions.Worktrees.Picked
	return req, nil
}
