package status

import (
	"cmp"
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

const (
	pathAll     = "all"
	pathWithPRs = "with_prs"
)

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktree, err = domain.One[string](path, values)
	case pathAll:
		next.All, err = domain.Yes(path, values)
	case pathWithPRs:
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
	case pathAll:
		return domain.SaidYes(r.All)
	case pathWithPRs:
		return domain.SaidYes(r.WithPRs)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Skips is why the field at path is not asked, empty when it is: the worktree, when every worktree is read.
func (r Request) Skips(path string) string {
	if path == domain.PathWorktree && r.All {
		return "--all reads every worktree"
	}
	return ""
}

// Questions is the request as a form: the worktrees it may read, the one it reads, why the worktree is not asked, and its problems.
type Questions struct {
	worktrees []domain.Option
	read      string
	skip      string
	problems  []domain.FieldError
}

// NewQuestions is the form of req over the worktrees of facts, its named worktree located at a path (or ""): each one an option, the one req reads its answer.
func NewQuestions(facts worktree.Repository, req Request, located string) Questions {
	q := Questions{skip: req.Skips(domain.PathWorktree)}
	worktrees := listed(facts.Snapshot)
	current := worktree.Holding(worktrees, req.Caller.Cwd, pathOf)
	for i, w := range worktrees {
		q.worktrees = append(q.worktrees, optionOf(w, i == current))
	}

	targets, err := Targets(facts.Snapshot, req, located)
	if failure, ok := errors.AsType[*domain.Error](err); ok {
		q.problems = failure.Fields
	}
	if !req.All && len(targets) == 1 {
		q.read = subjectOf(targets[0])
	}

	return q
}

// Form is the schema of the request: the worktree, a positional the picker asks, unless every worktree is read; whether to read them all; whether to ask GitHub for pull requests.
func (q Questions) Form() domain.Form {
	worktree := domain.SelectField{
		FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Flag: "<worktree>", Label: "Worktree", Help: "The worktree to read, by its branch, its path or part of its branch"},
		Value:     q.read,
		Options:   q.worktrees,
	}
	worktree.Skip = q.skip

	return domain.Form{Problems: q.problems, Fields: []domain.Field{
		worktree,
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: pathAll, Flag: "--all", Label: "All", Help: "Read every worktree of the repository"}},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: pathWithPRs, Flag: "--with-prs", Label: "Pull requests", Help: "Ask GitHub for the worktree's newest pull request, open, merged or closed, and an open one's checks and review (needs gh)"}},
	}}
}

// optionOf is w as the picker offers it: by its branch, or by its path on no branch, which its label says; badged main, or current when the caller stands in it.
func optionOf(w worktree.Worktree, current bool) domain.Option {
	option := domain.Option{Value: subjectOf(w)}
	if w.Branch() == "" {
		option.Label = w.Path + " (detached)"
	}
	switch {
	case current:
		option.Badge = "current"
	case w.Main:
		option.Badge = "main"
	}

	return option
}

// subjectOf names w as a read's subject: its branch, else its path.
func subjectOf(w worktree.Worktree) string {
	return cmp.Or(w.Branch(), w.Path)
}
