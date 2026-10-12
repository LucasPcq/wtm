package styles

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestAPlanAlignsItsValuesThenListsItsWarnings(t *testing.T) {
	got := ansi.Strip(RenderPlan(PlanParams{
		Rows:     []PlanRow{{Label: "Branches", Value: "feat/a, feat/b"}, {Label: "From", Value: "main"}},
		Warnings: []string{"feat/a already exists"},
	}))

	want := "Branches  feat/a, feat/b\nFrom      main\n! feat/a already exists"
	if got != want {
		t.Errorf("RenderPlan() =\n%s\nwant\n%s", got, want)
	}
}
