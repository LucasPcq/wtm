package form_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

func TestAProblemCapitalisesItsOwnWordsNeverAValue(t *testing.T) {
	cases := map[string]struct {
		Found domain.FieldError
		Want  string
	}{
		"a value first":  {Found: domain.FieldError{Template: "{value} already has a worktree", Params: domain.Params{"value": "feat/other"}}, Want: "feat/other already has a worktree"},
		"the path first": {Found: domain.FieldError{Template: "{path} is required"}, Want: "Branches is required"},
		"a word first":   {Found: domain.FieldError{Template: "branch {branch} is checked out", Params: domain.Params{"branch": "feat/x"}}, Want: "Branch feat/x is checked out"},
		"a quote first":  {Found: domain.FieldError{Template: "{problem}", Params: domain.Params{"problem": `"x" is not valid`}}, Want: `"x" is not valid`},
	}
	for name, c := range cases {
		assert.EqualError(t, form.Problem(c.Found, "Branches"), c.Want, name)
	}
}

func TestAnErrorReadsAsASentence(t *testing.T) {
	assert.EqualError(t, form.Sentence(errors.New("internal error")), "Internal error")
	assert.NoError(t, form.Sentence(nil))
}
