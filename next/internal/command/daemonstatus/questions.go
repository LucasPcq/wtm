package daemonstatus

import "github.com/LucasPcq/wtm/next/internal/domain"

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of daemon status: nothing, the daemon being the machine's one.
type Request struct{}

// Answer refuses every field: daemon status has none.
func (r Request) Answer(path string, _ []string) (Request, error) {
	return r, domain.Unknown(path)
}

// Values is nil: daemon status has no field.
func (Request) Values(string) []string {
	return nil
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: none.
type Questions struct{}

// Form is the schema of the request: no field.
func (Questions) Form() domain.Form {
	return domain.Form{}
}
