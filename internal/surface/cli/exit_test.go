package cli

import (
	"encoding/json"
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

func TestABatchExitsOnItsInterruptedItemFirstThenItsFirstFailure(t *testing.T) {
	first := kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed, Params: kernel.Params{kernel.ParamStep: "one"}})
	second := kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed, Params: kernel.Params{kernel.ParamStep: "two"}})
	done := kernel.Item[json.RawMessage]{Subject: "a", Status: kernel.StatusDone}
	failedOne := kernel.Item[json.RawMessage]{Subject: "b", Status: kernel.StatusFailed, Error: first}
	failedTwo := kernel.Item[json.RawMessage]{Subject: "c", Status: kernel.StatusFailed, Error: second}
	notReached := kernel.Item[json.RawMessage]{Subject: "d", Status: kernel.StatusSkipped, Reason: kernel.ReasonInterrupted}

	assert.Nil(t, outcomeFailure([]kernel.Item[json.RawMessage]{done}))
	assert.Equal(t, first, outcomeFailure([]kernel.Item[json.RawMessage]{done, failedOne, failedTwo}))
	assert.Equal(t, kernel.KindCancelled, outcomeFailure([]kernel.Item[json.RawMessage]{failedOne, notReached}).Kind())
}

func TestACancelledItemExitsOnItsOwnError(t *testing.T) {
	own := kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted, Params: kernel.Params{kernel.ParamPhase: "on_create"}})

	failure := outcomeFailure([]kernel.Item[json.RawMessage]{{Subject: "a", Status: kernel.StatusCancelled, Error: own}})

	assert.Equal(t, own, failure)
}
