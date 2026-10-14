package cli

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
)

// The exit codes of the kinds a command on the engine fails with, beyond those
// the legacy commands already have.
const (
	exitCodeRefused      = 23
	exitCodeConflict     = 24
	exitCodePrecondition = 25
)

// exitByCode lists the codes whose exit code is their own, not their kind's.
var exitByCode = map[kernel.Code]int{
	kernel.CodeStateSchemaNewer: domain.ExitCodeStateSchemaNewer,
}

// exitCode reads a failure of the engine by its code, the entry's own first,
// then by its kind; an error it did not classify, internal or from a legacy
// command, keeps the code of the sentinel it wraps.
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

// reported is a failure whose report is written already, on screen or in the
// envelope: Execute turns it into an exit code, and into its line only when
// --quiet threw the report away.
type reported struct {
	failure   kernel.Error
	line      string
	exitCodes map[kernel.Code]int
}

func (r *reported) Error() string { return r.line }

func (r *reported) Unwrap() []error { return []error{domain.ErrAborted, r.failure} }
