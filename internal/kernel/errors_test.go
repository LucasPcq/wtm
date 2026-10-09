package kernel_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestTheKindOfAnErrorIsItsType(t *testing.T) {
	problem := kernel.Problem{Code: "test.code"}
	cases := []struct {
		err  kernel.Error
		kind kernel.Kind
	}{
		{kernel.Invalid(nil), kernel.KindInvalid},
		{&kernel.RefusedError{Problem: problem}, kernel.KindRefused},
		{kernel.NotFound(problem), kernel.KindNotFound},
		{kernel.Conflict(problem), kernel.KindConflict},
		{kernel.Precondition(problem), kernel.KindPrecondition},
		{kernel.Cancelled(problem), kernel.KindCancelled},
		{kernel.Internal(problem), kernel.KindInternal},
	}
	for _, c := range cases {
		assert.Equal(t, c.kind, c.err.Kind())
	}
}

func TestAnInvalidRequestNamesEachFieldAtFault(t *testing.T) {
	problems := []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired}, {Path: "branches[1]", Code: kernel.CodeDistinct}}
	err := kernel.Invalid(problems)
	assert.Equal(t, kernel.CodeInvalidRequest, err.Code)
	assert.Equal(t, problems, err.Fields)
}

func TestAnErrorReadsAsItsCodeNeverAsASentence(t *testing.T) {
	assert.Equal(t, "lock.held", kernel.Conflict(kernel.Problem{Code: "lock.held"}).Error())
	assert.Equal(t, "unit.step_failed: boom", kernel.Internal(kernel.Problem{Code: kernel.CodeStepFailed, Cause: errBoom}).Error())
}

func TestAnErrorIsFoundThroughWrappingByItsTypeAndItsCause(t *testing.T) {
	refused := &kernel.RefusedError{Problem: kernel.Problem{Code: "test.dirty", Cause: errBoom}, Blockers: []kernel.Blocker{{Code: "test.dirty", Field: "force"}}}
	err := fmt.Errorf("clean: %w", refused)

	var asRefused *kernel.RefusedError
	require.ErrorAs(t, err, &asRefused)
	assert.Equal(t, "force", asRefused.Blockers[0].Field)

	var asError kernel.Error
	require.ErrorAs(t, err, &asError)
	assert.Equal(t, kernel.Code("test.dirty"), asError.Base().Code)

	assert.ErrorIs(t, err, errBoom)
}

func TestClassifySortsAnyErrorIntoTheTaxonomy(t *testing.T) {
	refused := &kernel.RefusedError{Problem: kernel.Problem{Code: "test.dirty"}}
	params := kernel.Params{kernel.ParamStep: "worktree"}
	cases := []struct {
		name     string
		err      error
		code     kernel.Code
		wantKind kernel.Kind
		wantCode kernel.Code
	}{
		{"an Error stays as it is, even wrapped", fmt.Errorf("x: %w", refused), kernel.CodeStepFailed, kernel.KindRefused, "test.dirty"},
		{"a cancellation is cancelled", fmt.Errorf("git: %w", context.Canceled), kernel.CodeStepFailed, kernel.KindCancelled, kernel.CodeInterrupted},
		{"anything else is internal, under the code given", errBoom, kernel.CodeStepFailed, kernel.KindInternal, kernel.CodeStepFailed},
		{"with no code given, internal", errBoom, "", kernel.KindInternal, kernel.CodeInternal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := kernel.Classify(kernel.ClassifyParams{Err: c.err, Code: c.code, Params: params})
			assert.Equal(t, c.wantKind, got.Kind())
			assert.Equal(t, c.wantCode, got.Base().Code)
			assert.True(t, errors.Is(got, c.err) || got == kernel.Error(refused), "the original error is kept")
		})
	}
}
