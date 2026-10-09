package inittui

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

// The step borrows the runner list; what goes through it has to come back as
// the same answers, or the write side applies someone else's rows.
func TestTouchRowsRoundTripThroughTheList(t *testing.T) {
	choices := []domain.JobTouchChoice{
		{Job: "orm:billing:reset", Label: "orm:billing:reset", Touches: []string{"postgres-billing"}, Options: []string{"", "postgres-billing"}},
		{Job: "build:shared", Label: "build:shared", Options: []string{"", "postgres-billing"}},
	}
	list := components.NewRunnerList(components.NewRunnerListParams{Choices: touchRows(choices), Help: domain.HelpSetTouch})

	back := touchChoicesFrom(list.Choices())
	if len(back) != 2 || back[0].Job != "orm:billing:reset" || !slices.Equal(back[0].Touches, []string{"postgres-billing"}) || back[1].Touches != nil {
		t.Errorf("round trip = %+v, want the rows unchanged", back)
	}
	if got := touchListSummary(list); got != "1 of 2 task(s) change a service's data" {
		t.Errorf("summary = %q", got)
	}
}
