package settle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestARedactedPlanCarriesNoValueReadFromAFile(t *testing.T) {
	redacted := Redact(redactedPlanFixture())

	for _, entry := range redacted.Entries {
		assert.Empty(t, entry.CurrentValue, entry.Key)
		assert.Empty(t, entry.NewValue, entry.Key)
		assert.Empty(t, entry.ForeignHost, entry.Key)
	}
	assert.Equal(t, StatusRewrite, redacted.Entries[1].Status)
	assert.Equal(t, 5432, redacted.Entries[1].Base)
}

func TestARedactedPlanKeepsTheValuesWtmWrites(t *testing.T) {
	redacted := Redact(redactedPlanFixture())

	assert.Equal(t, "app-feat-a", redacted.Owned[0].Value)
}

func TestRedactingLeavesThePlanItReadIntact(t *testing.T) {
	plan := redactedPlanFixture()

	Redact(plan)

	assert.Equal(t, redactedPlanFixture(), plan)
}
