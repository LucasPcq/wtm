package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestEachKindExitsOnItsCode(t *testing.T) {
	problem := kernel.Problem{Code: "some.code"}
	cases := []struct {
		err  kernel.Error
		code int
	}{
		{kernel.Invalid(nil), domain.ExitCodeUsage},
		{kernel.Cancelled(problem), domain.ExitCodeCancelled},
		{&kernel.RefusedError{Problem: problem}, exitCodeRefused},
		{kernel.Conflict(problem), exitCodeConflict},
		{kernel.Precondition(problem), exitCodePrecondition},
		{kernel.NotFound(problem), domain.ExitCodeError},
		{kernel.Internal(problem), domain.ExitCodeError},
	}
	for _, c := range cases {
		assert.Equal(t, c.code, exitCode(c.err), c.err.Kind())
	}
}

func TestACodeWithAnExitOfItsOwnExitsOnIt(t *testing.T) {
	newer := kernel.Precondition(kernel.Problem{Code: kernel.CodeStateSchemaNewer})

	assert.Equal(t, domain.ExitCodeStateSchemaNewer, exitCode(newer))
}

func TestTheEntrysOwnExitCodeComesBeforeTheEnginesAndTheKind(t *testing.T) {
	taken := kernel.Conflict(kernel.Problem{Code: codeBranchTaken})
	newer := kernel.Precondition(kernel.Problem{Code: kernel.CodeStateSchemaNewer})
	exitCodes := map[kernel.Code]int{codeBranchTaken: domain.ExitCodeWorktreeExists, kernel.CodeStateSchemaNewer: domain.ExitCodeError}

	assert.Equal(t, domain.ExitCodeWorktreeExists, exitCode(&reported{failure: taken, exitCodes: exitCodes}))
	assert.Equal(t, domain.ExitCodeError, exitCode(&reported{failure: newer, exitCodes: exitCodes}))
	assert.Equal(t, exitCodeConflict, exitCode(&reported{failure: taken}))
}

func TestAnInternalErrorKeepsTheCodeOfTheSentinelItWraps(t *testing.T) {
	notARepo := kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: fmt.Errorf("resolve: %w", domain.ErrNotGitRepo)})

	assert.Equal(t, domain.ExitCodeNotGitRepo, exitCode(notARepo))
}

func TestAnErrorOfALegacyCommandKeepsItsCode(t *testing.T) {
	assert.Equal(t, domain.ExitCodeBranchNotFound, exitCode(domain.ErrBranchNotFound))
	assert.Equal(t, domain.ExitCodeError, exitCode(errors.New("boom")))
}

func TestAReportedFailureExitsOnItsErrorAndReadsAsItsLine(t *testing.T) {
	err := error(&reported{failure: kernel.Conflict(kernel.Problem{Code: kernel.CodeLockHeld}), line: "repo is held"})

	assert.Equal(t, exitCodeConflict, exitCode(err))
	assert.ErrorIs(t, err, domain.ErrAborted)
	assert.Equal(t, "repo is held", abortLine(err))
}

func TestAnInterruptedRunExits19WhateverItsError(t *testing.T) {
	err := fmt.Errorf("%w: %w", domain.ErrCancelled, kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed}))

	assert.Equal(t, domain.ExitCodeCancelled, exitCode(err))
}
