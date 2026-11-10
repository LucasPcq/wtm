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

// The owner's rule: a row of commands or paths lists one per line under the values' column, whatever the width; a list of short names stays on one line.
func TestARowOfCommandsOrPathsListsOneValuePerLine(t *testing.T) {
	recap := []domain.RecapRow{
		{Label: "Branches", Values: domain.RecapValues([]string{"feat/a", "feat/b"})},
		{Label: ".env files", Stacked: true, Values: domain.RecapValues([]string{"apps/api/.env", "apps/web/.env"})},
		{Label: "On create", Stacked: true, Values: domain.RecapValues([]string{"pnpm install", "docker compose up -d"})},
		{Label: "On clean", Stacked: true, Values: domain.RecapValues([]string{"docker compose down"})},
	}

	lines := PlanLines(PlanParams{Rows: RecapRows(recap), Width: 100})

	assert.Equal(t, []string{
		"Branches    feat/a, feat/b",
		".env files  apps/api/.env",
		"            apps/web/.env",
		"On create   pnpm install",
		"            docker compose up -d",
		"On clean    docker compose down",
	}, lines)
}
