package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestRememberAcceptsOnlyTheRememberableFields(t *testing.T) {
	field := kernel.Remember(kernel.RememberParams[request, facts]{Fields: createFields(), Label: "Remember"})

	assert.Equal(t, kernel.RememberPath, field.Spec.Path)
	assert.Equal(t, kernel.FieldMultiSelect, field.Spec.Type)
	assert.Equal(t, []string{"isolation"}, field.Spec.Constraints.Enum)
	assert.Empty(t, checkSpec(t, request{Remember: []string{"isolation"}}, field.Spec))
	problems := checkSpec(t, request{Remember: []string{"from"}}, field.Spec)
	if assert.Len(t, problems, 1) {
		assert.Equal(t, kernel.CodeOneOf, problems[0].Code)
	}
}
