package rules

import "github.com/LucasPcq/wtm/internal/domain"

// Usage marks err as a command line refused before the command ran, keeping
// its message: domain.ErrUsage joins the chain rules.ExitCode reads.
func Usage(err error) error {
	if err == nil {
		return nil
	}
	return usageError{err: err}
}

type usageError struct{ err error }

func (e usageError) Error() string   { return e.err.Error() }
func (e usageError) Unwrap() []error { return []error{e.err, domain.ErrUsage} }
