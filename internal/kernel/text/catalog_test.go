package text_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

func TestAMessageFillsEveryParam(t *testing.T) {
	assert.Equal(t, "could not undo env, left behind: worktree,env",
		text.NewCatalog().Message(kernel.CodeUndoFailed, kernel.Params{kernel.ParamStep: "env", kernel.ParamLeft: "worktree,env"}))
}

func TestAnUnknownCodeReadsAsItself(t *testing.T) {
	assert.Equal(t, "nowhere.declared", text.NewCatalog().Message("nowhere.declared", nil))
}

func TestACatalogueSpeaksTheCodesOfEveryEntryBesideTheKernels(t *testing.T) {
	catalog := text.NewCatalog(text.Messages{codeBranchTaken: "{branch} is taken"})

	assert.Equal(t, "feat/a is taken", catalog.Message(codeBranchTaken, kernel.Params{"branch": "feat/a"}))
	assert.Equal(t, "internal error", catalog.Message(kernel.CodeInternal, nil))
}

func TestACodeSpelledTwiceIsRefused(t *testing.T) {
	assert.PanicsWithValue(t, `text: code "branch.taken" has two messages`, func() {
		text.NewCatalog(text.Messages{codeBranchTaken: "taken"}, text.Messages{codeBranchTaken: "in use"})
	})
	assert.PanicsWithValue(t, `text: code "internal" has two messages`, func() {
		text.NewCatalog(text.Messages{kernel.CodeInternal: "oops"})
	})
}

func TestAFieldErrorReadsAsASentence(t *testing.T) {
	cases := []struct {
		problem kernel.FieldError
		want    string
	}{
		{kernel.FieldError{Path: "from", Code: kernel.CodeRequired}, "from is required"},
		{kernel.FieldError{Path: "a", Code: kernel.CodeRequiredOneOf, With: []string{"b", "c"}}, "one of a, b, c is required"},
		{kernel.FieldError{Path: "push", Code: kernel.CodeExclusiveWith, With: []string{"no_push"}}, "push cannot be used with no_push"},
		{kernel.FieldError{Path: "mode", Code: kernel.CodeOneOf, Params: kernel.Params{kernel.ParamValue: "x"}, Accepted: []string{"a", "b"}}, "mode must be one of a, b, not x"},
		{kernel.FieldError{Path: "branches[2]", Code: kernel.CodeDistinct, Params: kernel.Params{kernel.ParamValue: "feat/x"}}, "branches[2]: feat/x is given twice"},
		{kernel.FieldError{Path: "from", Code: kernel.CodeRequired, Params: kernel.Params{kernel.ParamBecause: string(kernel.CodeInternal)}}, "from is required: internal error"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, text.NewCatalog().Field(c.problem))
	}
}

func TestEveryStatusAndReasonHasItsLabel(t *testing.T) {
	catalog := text.NewCatalog()
	statuses := map[kernel.Status]string{
		kernel.StatusDone: "done", kernel.StatusFailed: "failed", kernel.StatusSkipped: "skipped", kernel.StatusCancelled: "cancelled",
	}
	reasons := map[kernel.Reason]string{
		kernel.ReasonInterrupted: "interrupted", kernel.ReasonNotReached: "not reached", kernel.ReasonUnchanged: "unchanged", kernel.ReasonUnsafe: "unsafe", kernel.ReasonLocked: "locked",
	}
	for status, label := range statuses {
		assert.Equal(t, label, catalog.Status(status))
	}
	for reason, label := range reasons {
		assert.Equal(t, label, catalog.Reason(reason))
	}
}
