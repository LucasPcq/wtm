package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
)

func TestEachKindExitsOnItsCode(t *testing.T) {
	cases := map[wtm.Kind]int{
		wtm.KindInvalid:      exitUsage,
		wtm.KindCancelled:    exitCancelled,
		wtm.KindRefused:      exitRefused,
		wtm.KindConflict:     exitConflict,
		wtm.KindPrecondition: exitPrecondition,
		wtm.KindNotFound:     exitError,
		wtm.KindInternal:     exitError,
	}
	for kind, code := range cases {
		assert.Equal(t, code, exitCode(exitParams{Failure: &wtm.Error{Kind: kind, Code: "some.code"}}), kind)
	}
}

func TestACodeWithAnExitOfItsOwnExitsOnIt(t *testing.T) {
	assert.Equal(t, exitConfigNotFound, exitCode(exitParams{Failure: &wtm.Error{Kind: wtm.KindPrecondition, Code: "create.config_not_found"}}))
	assert.Equal(t, exitNotGitRepo, exitCode(exitParams{Failure: &wtm.Error{Kind: wtm.KindPrecondition, Code: codeNotARepository}}))
}

func TestAStateFileOfANewerSchemaExitsOnItsCodeWhereverItSurfaced(t *testing.T) {
	newer := &wtm.Error{Kind: wtm.KindInternal, Code: "unit.step_failed", Cause: &wtm.SchemaNewer{Path: "meta.json", Found: 3, Supported: 2}}

	assert.Equal(t, exitStateSchemaNewer, exitCode(exitParams{Failure: newer}))
}

func TestAFailureAfterAnInterruptIsTheInterrupt(t *testing.T) {
	assert.Equal(t, exitCancelled, exitCode(exitParams{Failure: &wtm.Error{Kind: wtm.KindInternal}, Signalled: true}))
	assert.Equal(t, exitOK, exitCode(exitParams{Signalled: true}))
}
