package render_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestAFailureNamesWhatToFixOneByOneThenWhatToRun(t *testing.T) {
	var out bytes.Buffer
	invalid := kernel.Invalid([]kernel.FieldError{
		{Path: "branches", Code: kernel.CodeRequired},
		{Path: "isolation", Code: kernel.CodeOneOf, Accepted: []string{"isolated", "verbatim"}, Params: kernel.Params{kernel.ParamValue: "copied"}},
	})

	render.Failure(&out, render.FailureParams{Err: invalid, Wording: wording, Next: []render.NextStepParams{{Command: "wtm make --help"}}})

	assert.Equal(t, lines(
		"  ✗ the request is invalid",
		"    --branches is required",
		"    --isolation must be one of isolated, verbatim, not copied",
		"  → wtm make --help",
	), plain(out.String()))
}

func TestAFailureLineHoldsTheWholeFailure(t *testing.T) {
	invalid := kernel.Invalid([]kernel.FieldError{{Path: "branches", Code: kernel.CodeRequired}, {Path: "from", Code: kernel.CodeRequired}})
	refused := &kernel.RefusedError{Problem: kernel.Problem{Code: kernel.CodeInterrupted}, Blockers: []kernel.Blocker{{Code: kernel.CodeLocksUnavailable, Field: "force"}}}
	conflict := kernel.Conflict(kernel.Problem{Code: kernel.CodePlanStale})

	assert.Equal(t, "the request is invalid: --branches is required; --from is required", wording.FailureLine(invalid))
	assert.Equal(t, "interrupted: ran without operation locks (--force)", wording.FailureLine(refused))
	assert.Equal(t, "the facts changed since the preview", wording.FailureLine(conflict))
}

func TestOnlyAnErrorWithNothingMorePreciseToSayKeepsItsCause(t *testing.T) {
	cause := errors.New("exit 128")

	assert.Equal(t, "internal error: exit 128", wording.Error(kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: cause})))
	assert.Equal(t, "the request is invalid: exit 128", wording.Error(&kernel.InvalidError{Problem: kernel.Problem{Code: kernel.CodeInvalidRequest, Cause: cause}}))
	assert.Equal(t, "the request is invalid", wording.Error(&kernel.InvalidError{Problem: kernel.Problem{Code: kernel.CodeInvalidRequest, Cause: cause}, Fields: []kernel.FieldError{{Path: "a", Code: kernel.CodeRequired}}}))
	assert.Equal(t, "interrupted", wording.Error(kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted, Cause: cause})))
	assert.Equal(t, "interrupted", wording.Error(&kernel.RefusedError{Problem: kernel.Problem{Code: kernel.CodeInterrupted, Cause: cause}}))
}
