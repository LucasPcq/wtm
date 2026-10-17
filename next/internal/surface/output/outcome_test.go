package output

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
)

func TestTwoBranchesCreatedAreCountedRowedAndPointedAt(t *testing.T) {
	trees := inTrees(t)

	golden(t, "outcome_two_done", rendered(func(w *bytes.Buffer) { Outcome(w, OutcomeParams{Outcome: twoCreated(trees)}) }))
}

func TestAMixedRunNamesEachAnomalyAndEachUnfinishedBranch(t *testing.T) {
	trees := inTrees(t)

	golden(t, "outcome_mixed", rendered(func(w *bytes.Buffer) {
		Outcome(w, OutcomeParams{Outcome: mixed(trees), FollowUps: []NextStep{{Command: "wtm clean feat/e --force", Note: "branch feat/e is left half-created at /trees/feat-e: remove it with `wtm clean --force feat/e`"}}})
	}))
}

func TestARunOfOnlyOpenBranchesIsUnchanged(t *testing.T) {
	trees := inTrees(t)
	outcome := twoCreated(trees)
	for i := range outcome.Result.Items {
		outcome.Result.Items[i].Reason = "unchanged"
		outcome.Plan.Branches[i].Action = create.ActionUnchanged
	}

	golden(t, "outcome_unchanged", rendered(func(w *bytes.Buffer) { Outcome(w, OutcomeParams{Outcome: outcome}) }))
}

func TestARunWithNothingToDoSaysSo(t *testing.T) {
	golden(t, "outcome_nothing", rendered(func(w *bytes.Buffer) { Outcome(w, OutcomeParams{Outcome: app.CreateOutcome{Result: &create.Result{}}}) }))
}
