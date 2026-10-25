package form_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

func TestEveryReasonAStepIsNotAskedHasItsSentence(t *testing.T) {
	for _, skip := range domain.Skips {
		assert.NotEmpty(t, form.Skipped(skip, domain.Form{}), skip)
	}
	assert.Empty(t, form.Skipped("", domain.Form{}))
}

func TestTheHeldFastForwardNamesTheBranchHoldingTheChanges(t *testing.T) {
	extracting := domain.Form{Fields: []domain.Field{domain.SelectField{FieldSpec: domain.FieldSpec{Path: "source"}, Value: "feat/login"}}}

	assert.Equal(t, "nothing to fast-forward: feat/login holds the changes being extracted", form.Skipped(domain.SkipHeld, extracting))
	assert.Equal(t, "nothing to fast-forward: the source holds the changes being extracted", form.Skipped(domain.SkipHeld, domain.Form{}))
}
