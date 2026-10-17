package cli

import (
	"cmp"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// The exit codes of wtm-next, shared with wtm: a script branches on them, so a value never changes meaning.
const (
	ExitOK               = 0
	ExitError            = 1  // a failure no other code names
	ExitUsage            = 2  // a flag or an argument the command refuses
	ExitConfigNotFound   = 12 // a command's own code for a missing config file, set in its Exits
	ExitCancelled        = 19 // interrupted by a signal or by the user leaving the form
	ExitNotGitRepo       = 21
	ExitStateSchemaNewer = 22 // a state file written by a newer wtm
	ExitRefused          = 23
	ExitConflict         = 24
	ExitPrecondition     = 25
)

var exitByCode = map[string]int{
	domain.CodeNotARepository: ExitNotGitRepo,
}

var exitByKind = map[domain.Kind]int{
	domain.KindInvalid:      ExitUsage,
	domain.KindCancelled:    ExitCancelled,
	domain.KindRefused:      ExitRefused,
	domain.KindConflict:     ExitConflict,
	domain.KindPrecondition: ExitPrecondition,
}

// exitCode returns the exit code of a command's failure, from the most specific source to the least: a signal, the command's own code, a code every command shares, a newer state file, then the failure's kind.
func exitCode(failed *Failed, signalled bool) int {
	if signalled {
		return ExitCancelled
	}

	failure := failed.Err
	if code, own := failed.Exits[failure.Code]; own {
		return code
	}
	if code, own := exitByCode[failure.Code]; own {
		return code
	}
	if _, newer := errors.AsType[*domain.SchemaNewer](failure); newer {
		return ExitStateSchemaNewer
	}

	return cmp.Or(exitByKind[failure.Kind], ExitError)
}
