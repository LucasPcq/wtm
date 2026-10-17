package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestEachKindExitsOnItsCode(t *testing.T) {
	cases := map[domain.Kind]int{
		domain.KindInvalid:      ExitUsage,
		domain.KindCancelled:    ExitCancelled,
		domain.KindRefused:      ExitRefused,
		domain.KindConflict:     ExitConflict,
		domain.KindPrecondition: ExitPrecondition,
		domain.KindInternal:     ExitError,
	}
	for kind, code := range cases {
		assert.Equal(t, code, exitCode(exitParams{Failed: &Failed{Err: &domain.Error{Kind: kind, Code: "some.code"}}}), kind)
	}
}

func TestACodeWithAnExitOfItsOwnExitsOnIt(t *testing.T) {
	assert.Equal(t, exitConfigNotFound, exitCode(exitParams{Failed: &Failed{Err: &domain.Error{Kind: domain.KindPrecondition, Code: codeConfigNotFound}, Exits: map[string]int{codeConfigNotFound: exitConfigNotFound}}}))
	assert.Equal(t, ExitNotGitRepo, exitCode(exitParams{Failed: &Failed{Err: &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository}}}))
}

func TestAStateFileOfANewerSchemaExitsOnItsCodeWhereverItSurfaced(t *testing.T) {
	newer := &domain.Error{Kind: domain.KindInternal, Code: "unit.step_failed", Cause: &domain.SchemaNewer{Path: "meta.json", Found: 3, Supported: 2}}

	assert.Equal(t, ExitStateSchemaNewer, exitCode(exitParams{Failed: &Failed{Err: newer}}))
}

func TestAFailureAfterAnInterruptIsTheInterrupt(t *testing.T) {
	assert.Equal(t, ExitCancelled, exitCode(exitParams{Failed: &Failed{Err: &domain.Error{Kind: domain.KindInternal}}, Signalled: true}))
}
