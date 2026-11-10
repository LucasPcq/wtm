package daemonstop

import "github.com/LucasPcq/wtm/next/internal/domain"

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of daemon stop: nothing, the daemon being the machine's one.
type Request struct{}

// Answer refuses every field: daemon stop has none.
func (r Request) Answer(path string, _ []string) (Request, error) {
	return r, domain.Unknown(path)
}

// Values is nil: daemon stop has no field.
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
