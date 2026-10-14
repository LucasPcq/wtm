package cli

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	exitCodeRefused      = 23
	exitCodeConflict     = 24
	exitCodePrecondition = 25
)

var exitByCode = map[kernel.Code]int{
	kernel.CodeStateSchemaNewer: domain.ExitCodeStateSchemaNewer,
}

func exitCode(err error) int {
	var failure kernel.Error
	if errors.Is(err, domain.ErrCancelled) || !errors.As(err, &failure) || failure.Kind() == kernel.KindInternal {
		return rules.ExitCode(err)
	}
	if code, own := entryExitCode(err); own {
		return code
	}
	if code, listed := exitByCode[failure.Base().Code]; listed {
		return code
	}
	return exitOfKind(failure.Kind())
}

func entryExitCode(err error) (int, bool) {
	var report *reported
	if !errors.As(err, &report) {
		return 0, false
	}
	code, own := report.exitCodes[report.failure.Base().Code]
	return code, own
}

func exitOfKind(kind kernel.Kind) int {
	switch kind {
	case kernel.KindInvalid:
		return domain.ExitCodeUsage
	case kernel.KindCancelled:
		return domain.ExitCodeCancelled
	case kernel.KindRefused:
		return exitCodeRefused
	case kernel.KindConflict:
		return exitCodeConflict
	case kernel.KindPrecondition:
		return exitCodePrecondition
	default:
		return domain.ExitCodeError
	}
}

type reported struct {
	failure   kernel.Error
	line      string
	exitCodes map[kernel.Code]int
}

func (r *reported) Error() string { return r.line }

func (r *reported) Unwrap() []error { return []error{domain.ErrAborted, r.failure} }
