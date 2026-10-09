package kernel_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAnErrorReadsAsItsCodeNeverAsASentence(t *testing.T) {
	bare := &kernel.Error{Kind: kernel.KindConflict, Code: "lock.held"}
	caused := &kernel.Error{Kind: kernel.KindInternal, Code: kernel.CodeStepFailed, Cause: errBoom}
	if bare.Error() != "lock.held" || caused.Error() != "unit.step_failed: boom" {
		t.Errorf("bare = %q, caused = %q", bare.Error(), caused.Error())
	}
}

func TestAnErrorUnwrapsToItsCause(t *testing.T) {
	err := fmt.Errorf("apply: %w", &kernel.Error{Kind: kernel.KindInternal, Code: kernel.CodeInternal, Cause: errBoom})
	if !errors.Is(err, errBoom) {
		t.Error("the cause is lost")
	}
	var known *kernel.Error
	if !errors.As(err, &known) || known.Code != kernel.CodeInternal {
		t.Errorf("errors.As = %+v", known)
	}
}

func TestInvalidIsThe422NamingEachField(t *testing.T) {
	problems := []kernel.FieldError{{Path: "from", Code: kernel.CodeRequired}, {Path: "branches[1]", Code: kernel.CodeDistinct}}
	err := kernel.Invalid(problems)
	if err.Kind != kernel.KindInvalid || err.Code != kernel.CodeInvalidRequest || len(err.Fields) != 2 {
		t.Errorf("Invalid = %+v", err)
	}
}

func TestClassifySortsAnyErrorIntoTheTaxonomy(t *testing.T) {
	refused := &kernel.Error{Kind: kernel.KindRefused, Code: "test.dirty"}
	params := map[string]string{kernel.ParamStep: "worktree"}
	cases := []struct {
		name     string
		err      error
		code     kernel.Code
		wantKind kernel.Kind
		wantCode kernel.Code
	}{
		{"an *Error stays as it is, even wrapped", fmt.Errorf("x: %w", refused), kernel.CodeStepFailed, kernel.KindRefused, "test.dirty"},
		{"a cancellation is cancelled", fmt.Errorf("git: %w", context.Canceled), kernel.CodeStepFailed, kernel.KindCancelled, kernel.CodeInterrupted},
		{"anything else is internal, under the code given", errBoom, kernel.CodeStepFailed, kernel.KindInternal, kernel.CodeStepFailed},
		{"with no code given, internal", errBoom, "", kernel.KindInternal, kernel.CodeInternal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := kernel.Classify(kernel.ClassifyParams{Err: c.err, Code: c.code, Params: params})
			if got.Kind != c.wantKind || got.Code != c.wantCode {
				t.Errorf("Classify = %s/%s, want %s/%s", got.Kind, got.Code, c.wantKind, c.wantCode)
			}
			if !errors.Is(got, c.err) && got != refused {
				t.Errorf("the original error is lost: %v", got)
			}
		})
	}
}
