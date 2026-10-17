package cli

import (
	"cmp"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/wtm"
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

const codeNotARepository = "git.not_a_repository"

var exitByCode = map[string]int{
	"create.config_not_found": exitConfigNotFound,
	codeNotARepository:        exitNotGitRepo,
}

var exitByKind = map[wtm.Kind]int{
	wtm.KindInvalid:      exitUsage,
	wtm.KindCancelled:    exitCancelled,
	wtm.KindRefused:      exitRefused,
	wtm.KindConflict:     exitConflict,
	wtm.KindPrecondition: exitPrecondition,
}

type exitParams struct {
	Failure   *wtm.Error
	Signalled bool
}

func exitCode(params exitParams) int {
	switch {
	case params.Failure == nil:
		return exitOK
	case params.Signalled:
		return exitCancelled
	}
	if code, own := exitByCode[params.Failure.Code]; own {
		return code
	}
	if _, newer := errors.AsType[*wtm.SchemaNewer](params.Failure); newer {
		return exitStateSchemaNewer
	}
	return cmp.Or(exitByKind[params.Failure.Kind], exitError)
}
