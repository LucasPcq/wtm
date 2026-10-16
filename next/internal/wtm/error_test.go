package wtm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnErrorIsClassifiedAsItIs(t *testing.T) {
	raised := &Error{Kind: KindConflict, Code: "lock.held", Message: "repo is held by wtm create (pid 7)"}

	classified := AsError(AsErrorParams{Err: fmt.Errorf("taking the lock: %w", raised), Code: "unit.step_failed"})

	assert.Same(t, raised, classified)
}

func TestACancellationIsCancelled(t *testing.T) {
	classified := AsError(AsErrorParams{Err: fmt.Errorf("git worktree add: %w", context.Canceled), Code: "unit.step_failed", Params: Params{"step": "add"}})

	assert.Equal(t, KindCancelled, classified.Kind)
	assert.Equal(t, CodeInterrupted, classified.Code)
	assert.Equal(t, "interrupted", classified.Message)
	assert.Equal(t, Params{"step": "add"}, classified.Params)
	assert.ErrorIs(t, classified, context.Canceled)
}

func TestAnyOtherErrorIsInternalAndKeepsItsCause(t *testing.T) {
	cause := errors.New("disk full")

	classified := AsError(AsErrorParams{Err: cause})

	assert.Equal(t, KindInternal, classified.Kind)
	assert.Equal(t, CodeInternal, classified.Code)
	assert.ErrorIs(t, classified, cause)
	assert.Equal(t, "internal error: disk full", classified.Error())
}

func TestACodeNamesTheFailureOfAnOpaqueCause(t *testing.T) {
	cause := errors.New("exit status 128")

	classified := AsError(AsErrorParams{Err: cause, Code: "unit.step_failed", Message: "add failed", Params: Params{"step": "add"}})

	assert.Equal(t, KindInternal, classified.Kind)
	assert.Equal(t, "unit.step_failed", classified.Code)
	assert.Equal(t, Params{"step": "add"}, classified.Params)
	assert.Equal(t, "add failed: exit status 128", classified.Error())
}

func TestNoErrorClassifiesAsNone(t *testing.T) {
	assert.Nil(t, AsError(AsErrorParams{}))
}

func TestAWrappedTypedNilIsAnInternalError(t *testing.T) {
	var none *Error

	classified := AsError(AsErrorParams{Err: wrapper{inner: none}})

	require.NotNil(t, classified)
	assert.Equal(t, KindInternal, classified.Kind)
	assert.Equal(t, CodeInternal, classified.Code)
}

func TestATypedNilErrorPrintsWithoutPanicking(t *testing.T) {
	var none *Error

	assert.Equal(t, "<nil *wtm.Error>", none.Error())
	assert.NoError(t, none.Unwrap())
}

func TestAnInvalidRequestListsItsFields(t *testing.T) {
	fields := []FieldError{{Path: "from", Code: "not_found", Message: "--from: ghost not found", Params: Params{"value": "ghost"}}}

	invalid := Invalid(fields)

	assert.Equal(t, KindInvalid, invalid.Kind)
	assert.Equal(t, CodeInvalidRequest, invalid.Code)
	assert.Equal(t, "the request is invalid", invalid.Error())
	assert.Equal(t, fields, invalid.Fields)
}
