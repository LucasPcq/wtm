package daemonstopjobs

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of stop-jobs: the repository whose jobs alone stop, by a path in it (none: every repository's), a relative one read from where the caller stands. Caller is where the caller stands; it is no field of the form: every surface sets it.
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
	if path == domain.PathRepo && r.Repo != "" {
		return []string{r.Repo}
	}
	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the repository, which only its flag sets, and its problem.
type Questions struct {
	Request Request
	Errors  []domain.FieldError
}

// Form is the questions as the schema a surface renders: the repository, flag only.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.TextField{FieldSpec: domain.FieldSpec{Path: domain.PathRepo, Skip: domain.SkipFlagOnly}, Value: q.Request.Repo},
	}}
}
