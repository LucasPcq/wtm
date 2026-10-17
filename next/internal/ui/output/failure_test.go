package output

import (
	"bytes"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestAnInvalidRequestNamesEachFieldAsTheCommandLineSpellsIt(t *testing.T) {
	golden(t, "failure_invalid", rendered(func(w *bytes.Buffer) { Failure(w, FailureParams{Err: invalid(), Flags: flags}) }))
}

func TestAStalePlanPointsAtThePlanTheFactsGiveNow(t *testing.T) {
	stale := &domain.Error{Kind: domain.KindPrecondition, Code: "plan.stale", Message: "the facts changed since the preview", Params: domain.Params{"expected": "a", "found": "b"}}

	golden(t, "failure_stale", rendered(func(w *bytes.Buffer) {
		Failure(w, FailureParams{Err: stale, Next: []NextStep{{Command: "wtm create feat/x --dry-run", Note: "the plan the facts give now"}}})
	}))
}

func TestAQuietFailureIsOneLineWithItsFields(t *testing.T) {
	internal := &domain.Error{Kind: domain.KindInternal, Code: "internal", Message: "internal error", Cause: errors.New("disk full")}
	notFound := &domain.Error{Kind: domain.KindPrecondition, Code: "create.config_not_found", Message: "no wtm config found — run `wtm init` first", Cause: errors.New("project config: config file not found")}

	assert.Equal(t, "the request is invalid: branch feat/x is checked out at /p — run `wtm go feat/x` to jump in; <branches>[1]: feat/x is given twice; --env-from must be one of example, main, parent, not copy; --from is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)", failureLine(FailureParams{Err: invalid(), Flags: flags}))
	assert.Equal(t, "internal error: disk full", failureLine(FailureParams{Err: internal}))
	assert.Equal(t, "no wtm config found — run `wtm init` first", failureLine(FailureParams{Err: notFound}))
}
