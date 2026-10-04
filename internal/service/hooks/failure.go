package hooks

import (
	"errors"
	"os/exec"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Failure names the hook that failed and how, for a reader that reports it
// rather than prints it.
type Failure struct {
	Cmd      string
	ExitCode *int
	Err      error
}

func (f Failure) Error() string   { return f.Err.Error() }
func (f Failure) Unwrap() []error { return []error{f.Err, domain.ErrHookFailed} }

type failureParams struct {
	Cmd     string
	Wrapped error
	Cause   error
}

func failureOf(params failureParams) Failure {
	failure := Failure{Cmd: params.Cmd, Err: params.Wrapped}
	var exit *exec.ExitError
	if errors.As(params.Cause, &exit) {
		code := exit.ExitCode()
		failure.ExitCode = &code
	}
	return failure
}
