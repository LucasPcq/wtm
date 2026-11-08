package runps

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of run ps: the one repository whose jobs it lists, by a path in it (none: every repository of the machine). Caller is where the caller stands; it is no field of the form: every surface sets it.
type Request struct {
	Repo   string        `json:"repo"`
	Caller domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// Answer is the request with the field at path answered by values.
func (r Request) Answer(path string, values []string) (Request, error) {
	if path != domain.PathRepo {
		return r, domain.Unknown(path)
	}

	r.Repo = strings.Join(values, "")
	return r, nil
}

// Values is the request's answer to the field at path, nil when it leaves the field to its default.
func (r Request) Values(path string) []string {
	if path == domain.PathRepo {
		return domain.Spelled(r.Repo)
	}

	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form; run ps asks nothing, so its form only names its flag.
type Questions struct{}

// Form is the schema of the request: the repository its list narrows to.
func (Questions) Form() domain.Form {
	return domain.Form{Fields: []domain.Field{
		domain.TextField{FieldSpec: domain.FieldSpec{Path: domain.PathRepo}},
	}}
}
