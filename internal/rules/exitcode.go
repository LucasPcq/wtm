package rules

import (
	"context"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ExitCode maps a command error to the process exit code. Granular codes let
// LLM agents branch on the precise failure cause; any unmapped error falls back
// to the generic ExitCodeError. A nil error maps to ExitCodeOK.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return domain.ExitCodeOK
	case errors.Is(err, domain.ErrCancelled), errors.Is(err, context.Canceled):
		return domain.ExitCodeCancelled
	case errors.Is(err, domain.ErrWorktreePathExists), errors.Is(err, domain.ErrWorktreeExists), errors.Is(err, domain.ErrWorktreeNameTaken):
		return domain.ExitCodeWorktreeExists
	case errors.Is(err, domain.ErrBranchNotFound):
		return domain.ExitCodeBranchNotFound
	case errors.Is(err, domain.ErrConfigNotFound):
		return domain.ExitCodeConfigNotFound
	case errors.Is(err, domain.ErrNotGitRepo):
		return domain.ExitCodeNotGitRepo
	case errors.Is(err, domain.ErrEventsSchemaNewer):
		return domain.ExitCodeEventsSchemaNewer
	case errors.Is(err, domain.ErrEnvDrift):
		return domain.ExitCodeEnvDrift
	case errors.Is(err, domain.ErrUsage):
		return domain.ExitCodeUsage
	case errors.Is(err, domain.ErrJobNotFound), errors.Is(err, domain.ErrProfileNotFound):
		return domain.ExitCodeNotDeclared
	case errors.Is(err, domain.ErrRunNotInitialized):
		return domain.ExitCodeRunNotInitialized
	case errors.Is(err, domain.ErrExtractConflict):
		return domain.ExitCodeExtractConflict
	case errors.Is(err, domain.ErrUpgradeFromSource), errors.Is(err, domain.ErrUpgradeNotWritable):
		return domain.ExitCodeUpgradeUnsupported
	default:
		return domain.ExitCodeError
	}
}

type InterruptedParams struct {
	Err       error
	Signalled bool
}

// Interrupted reads a failure that followed an interrupt as the interrupt: the
// git or hook error it surfaced through is how the cancellation got out, not
// what went wrong. A run that ended cleanly despite the signal keeps its nil.
func Interrupted(params InterruptedParams) error {
	if params.Err == nil || !params.Signalled || errors.Is(params.Err, domain.ErrCancelled) {
		return params.Err
	}
	return fmt.Errorf("%w: %w", domain.ErrCancelled, params.Err)
}

type BackedOutParams struct {
	Err error
	// Cancelled is the mark a command leaves when the user backed out of it.
	Cancelled bool
}

// BackedOut reads a run that failed after the user backed out of it — a
// picker escaped, then the runner's own ErrAborted — as the cancellation it is.
func BackedOut(params BackedOutParams) error {
	if params.Err == nil || !params.Cancelled || errors.Is(params.Err, domain.ErrCancelled) {
		return params.Err
	}
	return fmt.Errorf("%w: %w", params.Err, domain.ErrCancelled)
}
