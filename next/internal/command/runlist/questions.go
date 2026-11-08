package runlist

import (
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of run list: the worktree whose jobs and profiles it lists (its branch, or its path read from where the caller stands; none: the one the caller stands in). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Worktree string        `json:"worktree"`
	Caller   domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	if path != domain.PathWorktree {
		return r, domain.Unknown(path)
	}

	worktree, err := domain.One[string](path, values)
	if err != nil {
		return r, err
	}
	r.Worktree = worktree
	return r, nil
}

// Values is the request's answer to the field at path, nil when it leaves the field to its default.
func (r Request) Values(path string) []string {
	if path == domain.PathWorktree {
		return domain.Spelled(r.Worktree)
	}

	return nil
}

// query is how r names its worktree, located the path its name stands for.
func (r Request) query(located string) domain.WorktreeQuery {
	return domain.WorktreeQuery{Name: r.Worktree, Located: located, Cwd: r.Caller.Cwd}
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees it may read, the one it reads, and its problems.
type Questions struct {
	worktrees []domain.Option
	read      string
	problems  []domain.FieldError
}

// NewQuestions is the form of req over the worktrees of facts, its named worktree located at a path (or ""): each one an option, the one req reads its answer.
func NewQuestions(facts worktree.Repository, req Request, located string) Questions {
	q := Questions{worktrees: domain.WorktreeOptions(facts.Snapshot.MainFirst(), req.query(located))}
	target, err := domain.TargetWorktree(facts.Snapshot.MainFirst(), req.query(located))
	if failure, ok := errors.AsType[*domain.Error](err); ok {
		q.problems = failure.Fields
	}
	if err == nil {
		q.read = target.Name()
	}
	return q
}

// Form is the schema of the request: the worktree, a positional the picker asks.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.problems, Fields: []domain.Field{
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree}, Value: q.read, Options: q.worktrees},
	}}
}
