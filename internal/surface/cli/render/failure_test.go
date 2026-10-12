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

	render.Failure(&out, render.FailureParams{Err: invalid, Names: asFlag, Next: []render.NextStepParams{{Command: "wtm make --help"}}})

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

	assert.Equal(t, "the request is invalid: --branches is required; --from is required", render.FailureLine(invalid, asFlag))
	assert.Equal(t, "interrupted: ran without operation locks (--force)", render.FailureLine(refused, asFlag))
	assert.Equal(t, "the facts changed since the preview", render.FailureLine(conflict, asFlag))
}

func TestOnlyAnErrorWithNothingMorePreciseToSayKeepsItsCause(t *testing.T) {
	cause := errors.New("exit 128")

	assert.Equal(t, "internal error: exit 128", render.ErrorMessage(kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: cause})))
	assert.Equal(t, "the request is invalid: exit 128", render.ErrorMessage(&kernel.InvalidError{Problem: kernel.Problem{Code: kernel.CodeInvalidRequest, Cause: cause}}))
	assert.Equal(t, "the request is invalid", render.ErrorMessage(&kernel.InvalidError{Problem: kernel.Problem{Code: kernel.CodeInvalidRequest, Cause: cause}, Fields: []kernel.FieldError{{Path: "a", Code: kernel.CodeRequired}}}))
	assert.Equal(t, "interrupted", render.ErrorMessage(kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted, Cause: cause})))
	assert.Equal(t, "interrupted", render.ErrorMessage(&kernel.RefusedError{Problem: kernel.Problem{Code: kernel.CodeInterrupted, Cause: cause}}))
}
