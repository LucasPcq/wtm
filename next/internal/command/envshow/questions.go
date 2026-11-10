package envshow

import (
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of env show: the worktree whose .env files it reads (its branch, or its path read from where the caller stands; none: the one the caller stands in), and whether the values wtm does not write are shown (ShowValues). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktree   string        `json:"worktree"`
	ShowValues bool          `json:"show_values"`
	Caller     domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// PathShowValues is the path of the switch that shows the values; the worktree's is domain.PathWorktree.
const PathShowValues = "show_values"

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktree, err = domain.One[string](path, values)
	case PathShowValues:
		next.ShowValues, err = domain.Yes(path, values)
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
		return domain.Spelled(r.Worktree)
	case PathShowValues:
		return domain.SaidYes(r.ShowValues)
	}
	return nil
}

// query is how r names its worktree, located the path its name stands for.
func (r Request) query(located string) domain.WorktreeQuery {
	return domain.WorktreeQuery{Name: r.Worktree, Located: located, Cwd: r.Caller.Cwd}
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees it may read, the one it reads, whether the values are shown, and its problems.
type Questions struct {
	worktrees []domain.Option
	read      string
	show      bool
	problems  []domain.FieldError
}

// NewQuestions is the form of req over the worktrees of facts, its named worktree located at a path (or ""): each one on a branch an option, the one req reads its answer.
func NewQuestions(facts worktree.Repository, req Request, located string) Questions {
	worktrees := onBranch(facts.Snapshot)
	q := Questions{worktrees: domain.WorktreeOptions(worktrees, req.query(located)), show: req.ShowValues}
	target, err := target(worktrees, req, located)
	if failure, ok := errors.AsType[*domain.Error](err); ok {
		q.problems = failure.Fields
	}
	if err == nil {
		q.read = target.Name()
	}
	return q
}

// Form is the schema of the request: the worktree, a positional the picker asks, and whether the values are shown, a switch only its flag sets.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.problems, Fields: []domain.Field{
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree}, Value: q.read, Options: q.worktrees},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathShowValues, Skip: domain.SkipFlagOnly}, Value: q.show, NoFirst: true},
	}}
}

// target is the worktree req reads among worktrees: the one it names, else the one the caller stands in, else the main one.
func target(worktrees []worktree.Worktree, req Request, located string) (worktree.Worktree, error) {
	if len(worktrees) == 0 {
		return worktree.Worktree{}, domain.Invalid([]domain.FieldError{{Path: domain.PathWorktree, Code: domain.CodeNotFound, Template: "no worktree is on a branch"}})
	}
	return domain.TargetWorktree(worktrees, req.query(located))
}

// onBranch are the worktrees env show may read, the main one first: those on a branch, whose record and addresses their branch names.
func onBranch(snapshot worktree.Snapshot) []worktree.Worktree {
	var found []worktree.Worktree
	for _, w := range snapshot.MainFirst() {
		if w.Branch() != "" {
			found = append(found, w)
		}
	}
	return found
}
