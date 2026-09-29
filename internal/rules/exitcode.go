package rules

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ExitCode maps a command error to the process exit code. Granular codes let
// LLM agents branch on the precise failure cause; any unmapped error falls back
// to the generic ExitCodeError. A nil error maps to ExitCodeOK.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return domain.ExitCodeOK
	case errors.Is(err, domain.ErrWorktreePathExists), errors.Is(err, domain.ErrWorktreeExists), errors.Is(err, domain.ErrWorktreeNameTaken):
		return domain.ExitCodeWorktreeExists
	case errors.Is(err, domain.ErrBranchNotFound):
		return domain.ExitCodeBranchNotFound
	case errors.Is(err, domain.ErrConfigNotFound):
		return domain.ExitCodeConfigNotFound
	case errors.Is(err, domain.ErrJobNotFound):
		return domain.ExitCodeServiceNotFound
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
