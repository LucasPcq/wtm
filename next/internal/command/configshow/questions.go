package configshow

import "github.com/LucasPcq/wtm/next/internal/domain"

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of config show: nothing, the repository's config.toml being its subject.
type Request struct{}

// Answer refuses every field: config show has none.
func (r Request) Answer(path string, _ []string) (Request, error) {
	return r, domain.Unknown(path)
}

// Values is nil: config show has no field.
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
