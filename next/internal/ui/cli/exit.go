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

type exitParams struct {
	Failed    *Failed
	Signalled bool
}

func exitCode(params exitParams) int {
	if params.Signalled {
		return ExitCancelled
	}
	failure := params.Failed.Err
	if code, own := params.Failed.Exits[failure.Code]; own {
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
