package domain

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
)

func TestAValueOutsideItsChoicesIsAFieldError(t *testing.T) {
	accepted := []string{"a", "b"}

	assert.Nil(t, OneOf(OneOfParams{Path: "mode", Value: "a", Accepted: accepted}))
	assert.Nil(t, OneOf(OneOfParams{Path: "mode", Accepted: accepted}))
	assert.Equal(t, []FieldError{{Path: "mode", Code: CodeOneOf, Template: "{path} must be one of {accepted}, not {value}", Params: Params{"value": "c"}, Accepted: accepted}}, OneOf(OneOfParams{Path: "mode", Value: "c", Accepted: accepted}))
}

func TestNamesSpellsEachValueInOrder(t *testing.T) {
	assert.Equal(t, []string{"isolated", "verbatim"}, Names(project.Isolations()))
}
