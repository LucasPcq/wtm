package tree

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of tree: whether to ask GitHub for each branch's pull request.
type Request struct {
	WithPRs bool `json:"with_prs"`
}

// PathWithPRs is the path of tree's field, by which each surface words it.
const PathWithPRs = "with_prs"

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	if path != PathWithPRs {
		return r, domain.Unknown(path)
	}

	withPRs, err := domain.Yes(path, values)
	if err != nil {
		return r, err
	}
	r.WithPRs = withPRs
	return r, nil
}

// Values is the request's answer to the field at path, nil when it leaves the field to its default.
func (r Request) Values(path string) []string {
	if path == PathWithPRs {
		return domain.SaidYes(r.WithPRs)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form; tree asks nothing, so its form only names its flag.
type Questions struct{}

// Form is the schema of the request: whether to ask GitHub for pull requests.
func (Questions) Form() domain.Form {
	return domain.Form{Fields: []domain.Field{
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathWithPRs}},
	}}
}
