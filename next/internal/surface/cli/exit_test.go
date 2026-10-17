package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestEachKindExitsOnItsCode(t *testing.T) {
	cases := map[domain.Kind]int{
		domain.KindInvalid:      exitUsage,
		domain.KindCancelled:    exitCancelled,
		domain.KindRefused:      exitRefused,
		domain.KindConflict:     exitConflict,
		domain.KindPrecondition: exitPrecondition,
		domain.KindInternal:     exitError,
	}
	for kind, code := range cases {
		assert.Equal(t, code, exitCode(exitParams{Failure: &domain.Error{Kind: kind, Code: "some.code"}}), kind)
	}
}

func TestACodeWithAnExitOfItsOwnExitsOnIt(t *testing.T) {
	assert.Equal(t, exitConfigNotFound, exitCode(exitParams{Failure: &domain.Error{Kind: domain.KindPrecondition, Code: create.CodeConfigNotFound}}))
	assert.Equal(t, exitNotGitRepo, exitCode(exitParams{Failure: &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository}}))
}

func TestAStateFileOfANewerSchemaExitsOnItsCodeWhereverItSurfaced(t *testing.T) {
	newer := &domain.Error{Kind: domain.KindInternal, Code: "unit.step_failed", Cause: &domain.SchemaNewer{Path: "meta.json", Found: 3, Supported: 2}}

	assert.Equal(t, exitStateSchemaNewer, exitCode(exitParams{Failure: newer}))
}

func TestAFailureAfterAnInterruptIsTheInterrupt(t *testing.T) {
	assert.Equal(t, exitCancelled, exitCode(exitParams{Failure: &domain.Error{Kind: domain.KindInternal}, Signalled: true}))
}
