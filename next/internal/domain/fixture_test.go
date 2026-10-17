package domain

// wrapper is an error that wraps inner, so a test sees whether AsError looks through wrapping.
type wrapper struct{ inner error }

func (w wrapper) Error() string { return "wrapped" }
func (w wrapper) Unwrap() error { return w.inner }
