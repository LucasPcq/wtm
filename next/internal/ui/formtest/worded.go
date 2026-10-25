package formtest

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
)

// Worded checks that words give every field of the schema, the form of zero questions, its flag, its label and its help, a multi-select its problem of none picked, and every option of a closed select, and an option with no value, its label: a question cannot ship without its command-line words.
func Worded[Q form.Questions](t *testing.T, words form.Worder[Q]) {
	t.Helper()
	var zero Q
	for _, field := range zero.Form().Fields {
		said, path := words(zero, field), field.Spec().Path
		assert.NotEmpty(t, said.Flag, "the flag of %s", path)
		assert.NotEmpty(t, said.Label, "the label of %s", path)
		assert.NotEmpty(t, said.Help, "the help of %s", path)

		switch field := field.(type) {
		case domain.MultiSelectField:
			assert.NotEmpty(t, said.Empty, "the problem of %s answered with none", path)
		case domain.SelectField:
			for _, option := range field.Options {
				worded := field.Closed || option.Value == ""
				assert.True(t, !worded || said.Options[option.Value].Label != "", "the label of %s's option %q", path, option.Value)
			}
		case domain.TextField, domain.TextListField, domain.ConfirmField:
		}
	}
}
