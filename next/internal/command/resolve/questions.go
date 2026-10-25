package resolve

import (
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of resolve: the worktree to resolve, by its branch, its path read from where the caller stands, or part of its branch. Caller is where the caller stands, whose worktree is resolved when none is named; it is no field of the form: every surface sets it.
type Request struct {
	Worktree string        `json:"worktree"`
	Caller   domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// query is how req names its worktree, located the path its name stands for.
func (r Request) query(located string) domain.WorktreeQuery {
	return domain.WorktreeQuery{Name: r.Worktree, Located: located, Cwd: r.Caller.Cwd}
}

// Answer is the request with the field at path answered by values; resolve's one field is domain.PathWorktree.
func (r Request) Answer(path string, values []string) (Request, error) {
	if path != domain.PathWorktree {
		return r, domain.Unknown(path)
	}

	named, err := domain.One[string](path, values)
	if err != nil {
		return r, err
	}
	r.Worktree = named
	return r, nil
}

// Values is the request's answer to the field at path, nil when it leaves the field to its default.
func (r Request) Values(path string) []string {
	if path == domain.PathWorktree {
		return domain.Spelled(r.Worktree)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees it may resolve, the one it resolves, and its problems.
type Questions struct {
	worktrees []domain.Option
	resolved  string
	problems  []domain.FieldError
}

// NewQuestions is the form of req over the worktrees of facts, its named worktree located at a path (or ""): each one an option, or each one an ambiguous name matches, the one req resolves its answer.
func NewQuestions(facts worktree.Repository, req Request, located string) Questions {
	q := Questions{worktrees: domain.WorktreeOptions(facts.Snapshot.MainFirst(), req.query(located))}

	target, err := Target(facts.Snapshot, req, located)
	if failure, ok := errors.AsType[*domain.Error](err); ok {
		q.problems = failure.Fields
	}
	if err == nil {
		q.resolved = target.Name()
	}

	return q
}

// Form is the schema of the request: the worktree, a positional the picker asks.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.problems, Fields: []domain.Field{
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree}, Value: q.resolved, Options: q.worktrees},
	}}
}
