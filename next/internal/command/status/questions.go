package status

import (
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of status: the worktree to read (its branch, its path read from where the caller stands, or part of its branch), or every worktree, and whether to ask GitHub for pull requests. Caller is where the caller stands, whose worktree is read when none is named; it is no field of the form: every surface sets it.
type Request struct {
	Worktree string        `json:"worktree"`
	All      bool          `json:"all"`
	WithPRs  bool          `json:"with_prs"`
	Caller   domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of status's own fields, by which each surface words them; the worktree's is domain.PathWorktree.
const (
	PathAll     = "all"
	PathWithPRs = "with_prs"
)

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktree, err = domain.One[string](path, values)
	case PathAll:
		next.All, err = domain.Yes(path, values)
	case PathWithPRs:
		next.WithPRs, err = domain.Yes(path, values)
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
	case PathAll:
		return domain.SaidYes(r.All)
	case PathWithPRs:
		return domain.SaidYes(r.WithPRs)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Skips is why the field at path is not asked, empty when it is: the worktree, when every worktree is read.
func (r Request) Skips(path string) domain.Skip {
	if path == domain.PathWorktree && r.All {
		return domain.SkipAll
	}
	return ""
}

// Questions is the request as a form: the worktrees it may read, the one it reads, why the worktree is not asked, and its problems.
type Questions struct {
	worktrees []domain.Option
	read      string
	skip      domain.Skip
	problems  []domain.FieldError
}

// NewQuestions is the form of req over the worktrees of facts, its named worktree located at a path (or ""): each one an option, or each one an ambiguous name matches, the one req reads its answer.
func NewQuestions(facts worktree.Repository, req Request, located string) Questions {
	q := Questions{skip: req.Skips(domain.PathWorktree)}
	worktrees := facts.Snapshot.MainFirst()
	offered := worktrees
	// an ambiguous name is asked again among the worktrees it matches
	if matches := worktree.Named(worktrees, req.Worktree, located); len(matches) > 1 {
		offered = matches
	}
	q.worktrees = domain.WorktreeOptions(offered, req.Caller.Cwd)

	targets, err := Targets(facts.Snapshot, req, located)
	if failure, ok := errors.AsType[*domain.Error](err); ok {
		q.problems = failure.Fields
	}
	if !req.All && len(targets) == 1 {
		q.read = targets[0].Name()
	}

	return q
}

// Form is the schema of the request: the worktree, a positional the picker asks, unless every worktree is read; whether to read them all; whether to ask GitHub for pull requests.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.problems, Fields: []domain.Field{
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Skip: q.skip}, Value: q.read, Options: q.worktrees},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathAll}},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathWithPRs}},
	}}
}
