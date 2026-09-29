package run

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// The picker of `run list` already chose, so its dispatch answers every step
// unattended; the safety question of a start still reaches the reader at the
// terminal instead of turning into a refusal naming --force.
func TestDispatchAsksTheSafetyQuestionAndNothingElse(t *testing.T) {
	answers := &flowtest.ScriptedPrompter{}
	confirms := &flowtest.ScriptedPrompter{Confirmed: true}
	prompter := confirmingPrompter{answers: answers, confirms: confirms}

	if !prompter.Interactive() {
		t.Fatal("the dispatch reads as unattended, so a start is refused instead of asked")
	}
	confirmed, err := prompter.Confirm(flow.ConfirmParams{Title: "foreign data"})
	if err != nil || !confirmed || confirms.Confirms != 1 {
		t.Errorf("confirm = %v, %v (%d asked), want the reader asked", confirmed, err, confirms.Confirms)
	}
	if _, err := prompter.Ask(flow.Session{}); err != nil {
		t.Fatalf("ask: %v", err)
	}
	if confirms.Asked != nil {
		t.Error("a step reached the interactive prompter")
	}
}
