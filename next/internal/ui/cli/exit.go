package cli

import (
	"cmp"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	ExitOK               = 0
	ExitError            = 1
	ExitUsage            = 2
	ExitCancelled        = 19
	ExitNotGitRepo       = 21
	ExitStateSchemaNewer = 22
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
