package cli

import (
	"cmp"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	exitOK               = 0
	exitError            = 1
	exitUsage            = 2
	exitConfigNotFound   = 12
	exitCancelled        = 19
	exitNotGitRepo       = 21
	exitStateSchemaNewer = 22
	exitRefused          = 23
	exitConflict         = 24
	exitPrecondition     = 25
)

var exitByCode = map[string]int{
	create.CodeConfigNotFound: exitConfigNotFound,
	domain.CodeNotARepository: exitNotGitRepo,
}

var exitByKind = map[domain.Kind]int{
	domain.KindInvalid:      exitUsage,
	domain.KindCancelled:    exitCancelled,
	domain.KindRefused:      exitRefused,
	domain.KindConflict:     exitConflict,
	domain.KindPrecondition: exitPrecondition,
}

type exitParams struct {
	Failure   *domain.Error
	Signalled bool
}

func exitCode(params exitParams) int {
	if params.Signalled {
		return exitCancelled
	}
	if code, own := exitByCode[params.Failure.Code]; own {
		return code
	}
	if _, newer := errors.AsType[*domain.SchemaNewer](params.Failure); newer {
		return exitStateSchemaNewer
	}
	return cmp.Or(exitByKind[params.Failure.Kind], exitError)
}
