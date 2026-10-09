package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestEveryCodeDeclaredHasItsMessage(t *testing.T) {
	declared := declaredCodes(t)
	require.NotEmpty(t, declared, "found no kernel.Code constant: the scan is broken")
	for code, where := range declared {
		assert.Contains(t, catalog, code, "%s declares code %q with no entry in the catalogue", where, code)
	}
}

func TestTheCatalogueHoldsOnlyDeclaredCodes(t *testing.T) {
	declared := declaredCodes(t)
	for code := range catalog {
		assert.Contains(t, declared, code, "catalogue entry %q matches no declared code", code)
	}
}

func TestAMessageFillsEveryParam(t *testing.T) {
	assert.Equal(t, "could not undo env, left behind: worktree,env",
		Message(kernel.CodeUndoFailed, kernel.Params{kernel.ParamStep: "env", kernel.ParamLeft: "worktree,env"}))
}

func TestAnUnknownCodeReadsAsItself(t *testing.T) {
	assert.Equal(t, "nowhere.declared", Message("nowhere.declared", nil))
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
		assert.Equal(t, c.want, Field(c.problem))
	}
}
