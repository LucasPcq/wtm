package list

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of list: whether to ask GitHub for each worktree's pull request, and whether to fetch origin first, as the picker's refresh does.
type Request struct {
	WithPRs bool `json:"with_prs"`
	Fetch   bool `json:"fetch"`
}

const (
	pathWithPRs = "with_prs"
	pathFetch   = "fetch"
)

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case pathWithPRs:
		next.WithPRs, err = domain.Yes(path, values)
	case pathFetch:
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
	case pathWithPRs:
		return domain.SaidYes(r.WithPRs)
	case pathFetch:
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
		domain.ConfirmField{
			FieldSpec: domain.FieldSpec{Path: pathWithPRs, Flag: "--with-prs", Label: "Pull requests", Help: "Ask GitHub for each worktree's open pull request (needs gh)"},
		},
		domain.ConfirmField{
			FieldSpec: domain.FieldSpec{Path: pathFetch, Flag: "--fetch", Label: "Fetch", Help: "Fetch origin first, as the picker's refresh (r) does"},
		},
	}}
}
