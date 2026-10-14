package hooks

import (
	"errors"
	"os/exec"
)

// Failure names the hook that failed or was stopped, for a reader that
// reports it rather than prints it. Err carries which: domain.ErrHookFailed or
// domain.ErrHookStopped.
type Failure struct {
	Cmd      string
	ExitCode *int
	Err      error
}

func (f Failure) Error() string { return f.Err.Error() }
func (f Failure) Unwrap() error { return f.Err }

type failureParams struct {
	Cmd     string
	Wrapped error
	Cause   error
}

func failureOf(params failureParams) Failure {
	failure := Failure{Cmd: params.Cmd, Err: params.Wrapped}
	var exit *exec.ExitError
	// -1 is a hook killed by a signal: it never exited with a code.
	if errors.As(params.Cause, &exit) && exit.ExitCode() >= 0 {
		code := exit.ExitCode()
		failure.ExitCode = &code
	}
	return failure
}
