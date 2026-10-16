package styles

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func renderedPlan(params PlanParams) string { return ansi.Strip(RenderPlan(params)) }

func TestAPlanAlignsItsValuesThenSetsItsWarningsApart(t *testing.T) {
	got := renderedPlan(PlanParams{
		Rows:     []PlanRow{{Label: "Branches", Values: []string{"feat/a", "feat/b"}}, {Label: "From", Values: []string{"main"}}},
		Warnings: []string{"feat/a already exists"},
	})

	want := "Branches  feat/a, feat/b\nFrom      main\n\n! feat/a already exists"
	if got != want {
		t.Errorf("RenderPlan() =\n%s\nwant\n%s", got, want)
	}
}

func TestAListTooLongForItsLineTakesALinePerItem(t *testing.T) {
	got := renderedPlan(PlanParams{
		Rows:  []PlanRow{{Label: "Branches", Values: []string{"feat/authentication", "feat/billing", "fix/session"}}, {Label: "From", Values: []string{"main"}}},
		Width: 40,
	})

	want := "Branches  feat/authentication\n          feat/billing\n          fix/session\nFrom      main"
	if got != want {
		t.Errorf("RenderPlan() =\n%s\nwant\n%s", got, want)
	}
}

func TestALongValueWrapsUnderTheColumnOfTheValues(t *testing.T) {
	got := renderedPlan(PlanParams{
		Rows:  []PlanRow{{Label: "Note", Values: []string{"the source is fast-forwarded first"}}},
		Width: 30,
	})

	want := "Note  the source is\n      fast-forwarded first"
	if got != want {
		t.Errorf("RenderPlan() =\n%s\nwant\n%s", got, want)
	}
}

func TestAPlanWithNoRowHasNothingToDo(t *testing.T) {
	if got := renderedPlan(PlanParams{}); got != "= nothing to do" {
		t.Errorf("RenderPlan() = %q, want %q", got, "= nothing to do")
	}
}

func TestARecapBodyIsTheBoxLessItsChrome(t *testing.T) {
	if got := RecapBodyWidth(IntroParams{Width: 80}); got != 70 {
		t.Errorf("RecapBodyWidth() = %d, want 70", got)
	}
}

func TestAValueLongerThanItsLineIsCutHard(t *testing.T) {
	got := renderedPlan(PlanParams{
		Rows:  []PlanRow{{Label: "To", Values: []string{"feat/a-very-long-branch"}}},
		Width: 14,
	})

	want := "To  feat/a-ver\n    y-long-bra\n    nch"
	if got != want {
		t.Errorf("RenderPlan() =\n%s\nwant\n%s", got, want)
	}
}

func TestAWarningOnSeveralLinesKeepsItsLinesUnderItsText(t *testing.T) {
	got := renderedPlan(PlanParams{
		Rows:     []PlanRow{{Label: "From", Values: []string{"main"}}},
		Warnings: []string{"ports in anomaly\nPORT   3000  no such key\nAPI    3001  shared"},
	})

	want := "From  main\n\n! ports in anomaly\n  PORT   3000  no such key\n  API    3001  shared"
	if got != want {
		t.Errorf("RenderPlan() =\n%s\nwant\n%s", got, want)
	}
}
