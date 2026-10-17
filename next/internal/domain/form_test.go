package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFirstFieldAnsweringAPathSpellsItsFlagAndUsage(t *testing.T) {
	form := Form{Fields: []Field{
		SelectField{FieldSpec: FieldSpec{Path: "to", Flag: "--to", Help: "Where to move", Usage: "Target branch"}},
		TextField{FieldSpec: FieldSpec{Path: "to", Flag: "--branch", Help: "Name for the branch"}},
		ConfirmField{FieldSpec: FieldSpec{Path: "keep", Flag: "--keep", Help: "Copy rather than move"}},
	}}

	assert.Equal(t, map[string]string{"to": "--to", "keep": "--keep"}, form.Flags())
	assert.Equal(t, map[string]string{"to": "Target branch", "keep": "Copy rather than move"}, form.Usages())
}

func TestOnlyASelectOrAConfirmationIsRemembered(t *testing.T) {
	form := Form{Fields: []Field{
		TextListField{FieldSpec: FieldSpec{Path: "branches"}},
		SelectField{FieldSpec: FieldSpec{Path: "isolation"}, Rememberable: true, Remember: true},
		SelectField{FieldSpec: FieldSpec{Path: "from"}},
		ConfirmField{FieldSpec: FieldSpec{Path: "fast_forward"}, Rememberable: true},
	}}

	assert.Equal(t, []string{"isolation", "fast_forward"}, form.Rememberable())
	assert.Equal(t, []string{"isolation"}, form.Remembering())
}

func TestAFieldsProblemsAreThoseOfItAndOfItsEntries(t *testing.T) {
	problems := []FieldError{{Path: "branches"}, {Path: "branches[1]"}, {Path: "branches_extra"}, {Path: "from"}}

	assert.Equal(t, []FieldError{{Path: "branches"}, {Path: "branches[1]"}}, ProblemsOf(problems, "branches"))
}

func TestEachKindAnswersAsItsCommandsRequestTakesIt(t *testing.T) {
	assert.Equal(t, []string{"main"}, SelectField{Value: "main"}.Answer())
	assert.Equal(t, []string{"a", "b"}, MultiSelectField{Picked: []string{"a", "b"}}.Answer())
	assert.Equal(t, []string{"feat/x"}, TextField{Value: "feat/x"}.Answer())
	assert.Equal(t, []string{"x", "y"}, TextListField{Entries: []string{"x", "y"}}.Answer())
	assert.Equal(t, []string{"true"}, ConfirmField{Value: true}.Answer())
}

func TestAConfirmationTakesTrueOrFalseOnly(t *testing.T) {
	yes, err := Yes(AnswerParams{Path: "keep", Values: ConfirmField{Value: true}.Answer()})
	require.NoError(t, err)
	assert.True(t, yes)

	for _, values := range [][]string{{"yes"}, {""}, {}, {"true", "false"}} {
		_, err := Yes(AnswerParams{Path: "keep", Values: values})
		failure, ok := errors.AsType[*Error](err)
		require.True(t, ok, values)
		assert.Equal(t, "keep", failure.Fields[0].Path, values)
	}
}
