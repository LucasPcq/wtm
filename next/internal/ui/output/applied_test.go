package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestAnAppliedOutcomeDrawsEachItemsWarningsBesideItsSubjectThenTheRuns(t *testing.T) {
	outcome := domain.Outcome[recappedPlan, worktreeDetail]{
		Plan:     recappedPlan{domain.Recapped{Recap: []domain.RecapRow{{Label: "From", Values: domain.RecapValues([]string{"main"})}}}},
		Warnings: []domain.Warning{{Code: "run", Message: "of the run"}},
		Result: &domain.Result[worktreeDetail]{Items: []domain.Unit[worktreeDetail]{
			{Subject: "feat/a", Status: domain.StatusDone, Warnings: []domain.Warning{{Code: "a", Message: "of feat/a"}}, Detail: &worktreeDetail{path: "/trees/feat-a"}},
			{Subject: "feat/b", Status: domain.StatusDone, Reason: domain.ReasonUnchanged, Detail: &worktreeDetail{path: "/trees/feat-b"}},
			{Subject: "feat/c", Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted, Warnings: []domain.Warning{{Code: "c", Message: "of feat/c"}}},
		}},
	}

	got := rendered(func(w *bytes.Buffer) {
		DrawApplied(w, outcome, AppliedParams{Command: "create", Unchanged: "already open", View: View{Cwd: "/trees"}})
	})

	assert.Equal(t, `  ! create: 1 done · 1 unchanged · 1 skipped

  From    main
  feat/a  feat-a
  feat/b  feat-b · already open

  ! feat/a  of feat/a
  ! of the run

  ! feat/c  interrupted
  ! feat/c  of feat/c
`, got)
}

func TestAnItemThePlanRuledOutIsCountedButNotDrawnAsUnfinished(t *testing.T) {
	outcome := domain.Outcome[recappedPlan, worktreeDetail]{
		Warnings: []domain.Warning{{Code: "left", Message: "feat/a has uncommitted changes: left as it is"}},
		Result:   &domain.Result[worktreeDetail]{Items: []domain.Unit[worktreeDetail]{{Subject: "feat/a", Status: domain.StatusSkipped, Reason: domain.ReasonRuledOut}}},
	}

	got := rendered(func(w *bytes.Buffer) { DrawApplied(w, outcome, AppliedParams{Command: "fast-forward"}) })

	assert.Equal(t, "  ! fast-forward: 1 skipped\n\n  ! feat/a has uncommitted changes: left as it is\n", got)
}

func TestAnAppliedOutcomeWithNoItemDrawsItsTallyAndTheRunsWarnings(t *testing.T) {
	outcome := domain.Outcome[recappedPlan, worktreeDetail]{Warnings: []domain.Warning{{Code: "run", Message: "of the run"}}, Result: &domain.Result[worktreeDetail]{}}

	got := rendered(func(w *bytes.Buffer) { DrawApplied(w, outcome, AppliedParams{Command: "extract", Note: "noted"}) })

	assert.Equal(t, "  = extract: nothing to do · noted\n\n  ! of the run\n", got)
}

func TestAStepTheWarningsOfEveryItemOfferIsDrawnOnceAfterTheItemsThenTheRuns(t *testing.T) {
	proxy := domain.Warning{Code: "proxy", Message: "proxy off", FollowUp: &domain.FollowUp{Command: "run proxy status", Message: "says why"}}
	outcome := domain.Outcome[recappedPlan, worktreeDetail]{
		Warnings: []domain.Warning{{Code: "run", Message: "of the run", FollowUp: &domain.FollowUp{Command: "env feat/a"}}},
		Result: &domain.Result[worktreeDetail]{Items: []domain.Unit[worktreeDetail]{
			{Subject: "feat/a", Status: domain.StatusDone, Warnings: []domain.Warning{proxy}},
			{Subject: "feat/b", Status: domain.StatusDone, Warnings: []domain.Warning{proxy}},
		}},
	}

	got := rendered(func(w *bytes.Buffer) {
		DrawApplied(w, outcome, AppliedParams{Command: "create", View: View{CommandLine: func(f domain.FollowUp) []string { return append([]string{"wtm"}, strings.Fields(f.Command)...) }}})
	})

	assert.Contains(t, got, "  → wtm run proxy status   says why\n  → wtm env feat/a\n")
	assert.Equal(t, 1, strings.Count(got, "run proxy status"))
}
