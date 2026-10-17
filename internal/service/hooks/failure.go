package hooks

import (
	"errors"
	"os/exec"
)

const signalledExitCode = -1

// Failure names the hook; Err is domain.ErrHookFailed or domain.ErrHookStopped.
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
	// os/exec gives a hook killed by a signal a negative code: it never exited with one.
	if errors.As(params.Cause, &exit) && exit.ExitCode() > signalledExitCode {
		code := exit.ExitCode()
		failure.ExitCode = &code
	}
	return failure
}
