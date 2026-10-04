package orphans

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

const key = "test.reparent"

var (
	ontoMain = []domain.ReparentResult{
		{Branch: "child", OldParent: "feat", NewParent: "main"},
		{Branch: "other", OldParent: "feat", NewParent: "main"},
	}
	ontoTwo = []domain.ReparentResult{
		{Branch: "leaf", OldParent: "mid", NewParent: "dev"},
		{Branch: "child", OldParent: "feat", NewParent: "main"},
	}
)

func stepFor(moves []domain.ReparentResult) flow.Step {
	return Step(StepParams{Key: key, Moves: func(flow.Answers) []domain.ReparentResult { return moves }})
}

func TestTheStepIsSkippedWhenNothingIsOrphaned(t *testing.T) {
	skip, reason := stepFor(nil).Skip(flow.Answers{})
	if !skip || reason != domain.NoOrphanedChildren {
		t.Errorf("skip = %v (%q), want skipped as having no orphaned children", skip, reason)
	}
}

func TestTheStepNamesEveryMoveAndASoleDestination(t *testing.T) {
	content, err := stepFor(ontoMain).Build(flow.Answers{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"child", "other", "feat", "main"} {
		if !strings.Contains(content.Description, want) {
			t.Errorf("proposal missing %q:\n%s", want, content.Description)
		}
	}
	if content.Options[0].Label != "Reparent onto main (2)" {
		t.Errorf("option = %q, want the one destination named", content.Options[0].Label)
	}

	many, _ := stepFor(ontoTwo).Build(flow.Answers{})
	if !strings.Contains(many.Options[0].Label, "nearest surviving ancestor") {
		t.Errorf("option = %q, want the destinations left to the proposal", many.Options[0].Label)
	}
}

// Reparenting is opt-in: `wtm reparent` can still move the children afterwards.
func TestAnUnattendedRunLeavesTheChildrenWhereTheyAre(t *testing.T) {
	answer, err := stepFor(ontoMain).Resolve(flow.Answers{})
	if err != nil || answer.Value != Orphan {
		t.Errorf("answer = %+v, err = %v, want the children left orphaned", answer, err)
	}
}

func TestPresetAnswersOnlyAnOptIn(t *testing.T) {
	if Preset(true) != Reparent || Preset(false) != "" {
		t.Errorf("Preset(true) = %q, Preset(false) = %q", Preset(true), Preset(false))
	}
}

func TestRecapLine(t *testing.T) {
	cases := []struct {
		name   string
		params RecapLineParams
		want   string
	}{
		{"nothing to say", RecapLineParams{}, ""},
		{"orphaned", RecapLineParams{Moves: ontoMain}, "Then leave 2 child worktree(s) orphaned."},
		{"one destination", RecapLineParams{Moves: ontoMain, Reparent: true}, "Then reparent 2 child worktree(s) onto main."},
		{"several", RecapLineParams{Moves: ontoTwo, Reparent: true}, "Then reparent 2 child worktree(s) onto their nearest surviving ancestor."},
	}
	for _, c := range cases {
		if got := RecapLine(c.params); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// The proposal names each child, where it lands and what it leaves — the recap
// only ever states a count, so this list is the one place the moves are legible.
func TestReparentProposalListsEveryMove(t *testing.T) {
	proposal := proposal([]domain.ReparentResult{
		{Branch: "child-a", OldParent: "parent-wt", NewParent: "main"},
		{Branch: "child-b", OldParent: "parent-wt", NewParent: "main"},
	})

	if !strings.HasPrefix(proposal, domain.ReparentIntro) {
		t.Errorf("proposal must open with its intro:\n%s", proposal)
	}
	for _, name := range []string{"child-a", "child-b", "parent-wt", "main"} {
		if !strings.Contains(proposal, name) {
			t.Errorf("proposal must name %q:\n%s", name, proposal)
		}
	}
	if lines := strings.Count(proposal, "\n"); lines != 2 {
		t.Errorf("proposal has %d line breaks, want one line per move under the intro:\n%s", lines, proposal)
	}
}
