package form_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

func TestEveryReasonAStepIsNotAskedHasItsSentence(t *testing.T) {
	for _, skip := range domain.Skips {
		assert.NotEmpty(t, form.Skipped(skip), skip)
	}
	assert.Empty(t, form.Skipped(""))
}
