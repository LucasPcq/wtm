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

	classified := AsError(fmt.Errorf("taking the lock: %w", raised))

	assert.Same(t, raised, classified)
}

func TestACancellationIsCancelled(t *testing.T) {
	classified := AsError(fmt.Errorf("git worktree add: %w", context.Canceled))

	assert.Equal(t, KindCancelled, classified.Kind)
	assert.Equal(t, CodeInterrupted, classified.Code)
	assert.Equal(t, "interrupted", classified.Message)
	assert.ErrorIs(t, classified, context.Canceled)
}

func TestAnyOtherErrorIsInternalAndKeepsItsCause(t *testing.T) {
	cause := errors.New("disk full")

	classified := AsError(cause)

	assert.Equal(t, KindInternal, classified.Kind)
	assert.Equal(t, CodeInternal, classified.Code)
	assert.ErrorIs(t, classified, cause)
	assert.Equal(t, "internal error: disk full", classified.Error())
}

func TestNoErrorClassifiesAsNone(t *testing.T) {
	assert.Nil(t, AsError(nil))
}

func TestAnInvalidRequestListsItsFields(t *testing.T) {
	fields := []FieldError{{Path: "from", Code: "not_found", Message: "--from: ghost not found", Params: Params{"value": "ghost"}}}

	invalid := Invalid(fields)

	require.NotNil(t, invalid)
	assert.Equal(t, KindInvalid, invalid.Kind)
	assert.Equal(t, CodeInvalidRequest, invalid.Code)
	assert.Equal(t, "the request is invalid", invalid.Error())
	assert.Equal(t, fields, invalid.Fields)
}
