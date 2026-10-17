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
		assert.Equal(t, code, exitCode(&Failed{Err: &domain.Error{Kind: kind, Code: "some.code"}}, false), kind)
	}
}

func TestACodeWithAnExitOfItsOwnExitsOnIt(t *testing.T) {
	assert.Equal(t, exitConfigNotFound, exitCode(&Failed{Err: &domain.Error{Kind: domain.KindPrecondition, Code: codeConfigNotFound}, Exits: map[string]int{codeConfigNotFound: exitConfigNotFound}}, false))
	assert.Equal(t, ExitNotGitRepo, exitCode(&Failed{Err: &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository}}, false))
}

func TestAStateFileOfANewerSchemaExitsOnItsCodeWhereverItSurfaced(t *testing.T) {
	newer := &domain.Error{Kind: domain.KindInternal, Code: "unit.step_failed", Cause: &domain.SchemaNewer{Path: "meta.json", Found: 3, Supported: 2}}

	assert.Equal(t, ExitStateSchemaNewer, exitCode(&Failed{Err: newer}, false))
}

func TestAFailureAfterAnInterruptIsTheInterrupt(t *testing.T) {
	assert.Equal(t, ExitCancelled, exitCode(&Failed{Err: &domain.Error{Kind: domain.KindInternal}}, true))
}
