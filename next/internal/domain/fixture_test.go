package domain

type wrapper struct{ inner error }

func (w wrapper) Error() string { return "wrapped" }
func (w wrapper) Unwrap() error { return w.inner }
