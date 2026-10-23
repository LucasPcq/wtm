package output

import (
	"bytes"
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
		DrawApplied(w, outcome, AppliedParams{Command: "create", View: View{Cwd: "/trees"}})
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

func TestAnAppliedOutcomeWithNoItemDrawsItsTallyAndTheRunsWarnings(t *testing.T) {
	outcome := domain.Outcome[recappedPlan, worktreeDetail]{Warnings: []domain.Warning{{Code: "run", Message: "of the run"}}, Result: &domain.Result[worktreeDetail]{}}

	got := rendered(func(w *bytes.Buffer) { DrawApplied(w, outcome, AppliedParams{Command: "extract", Note: "noted"}) })

	assert.Equal(t, "  = extract: nothing to do · noted\n\n  ! of the run\n", got)
}
