package output

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
)

func TestAPlanReadsAsItsRowsThenItsWarnings(t *testing.T) {
	plan := twoCreated(trees).Plan

	text := PlanText(PlanTextParams{Plan: plan, Warnings: []wtm.Warning{{Code: "create.ff_failed", Message: "couldn't fast-forward main"}}})

	assert.Equal(t, "Branches  feat/a, feat/b\nFrom      main\nEnv       example\n\n! couldn't fast-forward main", text)
}
