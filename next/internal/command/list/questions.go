package list

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of list: whether to ask GitHub for each worktree's pull request, and whether to fetch origin first, as the picker's refresh does. Caller is where the caller stands, whose worktree is the active one; it is no field of the form: every surface sets it.
type Request struct {
	WithPRs bool          `json:"with_prs"`
	Fetch   bool          `json:"fetch"`
	Caller  domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of list's fields, by which each surface words them.
const (
	PathWithPRs = "with_prs"
	PathFetch   = "fetch"
)

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case PathWithPRs:
		next.WithPRs, err = domain.Yes(path, values)
	case PathFetch:
		next.Fetch, err = domain.Yes(path, values)
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
	case PathWithPRs:
		return domain.SaidYes(r.WithPRs)
	case PathFetch:
		return domain.SaidYes(r.Fetch)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form; list asks nothing, so its form only names its flags.
type Questions struct{}

// Form is the schema of the request: whether to ask GitHub for pull requests, and whether to fetch origin first.
func (Questions) Form() domain.Form {
	return domain.Form{Fields: []domain.Field{
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathWithPRs}},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathFetch}},
	}}
}
