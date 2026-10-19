package output

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestTheMarkOfARowFollowsItsLastValue(t *testing.T) {
	recap := []domain.RecapRow{
		{Label: "Fast-forward", Mark: domain.MarkSaved, Values: domain.RecapValues([]string{"main", "topic"})},
		{Label: "Env", Mark: domain.MarkRecalled, Values: []domain.RecapValue{{Value: "example", Note: "config_default", Text: "config default (example)"}}},
		{Label: "From", Values: domain.RecapValues([]string{"main"})},
	}

	assert.Equal(t, []PlanRow{
		{Label: "Fast-forward", Values: []string{"main", "topic · saved for next runs"}},
		{Label: "Env", Values: []string{"config default (example) · remembered"}},
		{Label: "From", Values: []string{"main"}},
	}, RecapRows(recap))
}
