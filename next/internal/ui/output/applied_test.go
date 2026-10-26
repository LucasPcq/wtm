package output

import (
	"bytes"
	"strconv"
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

func TestAnItemThatDidNotEndDoneShowsTheLastLinesItPrintedAndItsLog(t *testing.T) {
	var tail []string
	for i := range 12 {
		tail = append(tail, "line "+strconv.Itoa(i+1))
	}
	outcome := domain.Outcome[recappedPlan, excerptDetail]{Result: &domain.Result[excerptDetail]{Items: []domain.Unit[excerptDetail]{
		{Subject: "feat/a", Status: domain.StatusDone, Detail: &excerptDetail{tail: []string{"fine"}, log: "/logs/feat-a.log"}},
		{Subject: "feat/b", Status: domain.StatusFailed, Err: &domain.Error{Message: "exited 3"}, Detail: &excerptDetail{tail: tail, log: "/logs/feat-b.log"}},
	}}}

	got := rendered(func(w *bytes.Buffer) { DrawApplied(w, outcome, AppliedParams{Command: "exec"}) })

	assert.NotContains(t, got, "fine")
	assert.NotContains(t, got, "line 2\n")
	assert.Contains(t, got, "  ✗ feat/b  exited 3\n      line 3\n")
	assert.Contains(t, got, "      line 12\n  full output:  /logs/feat-b.log\n")
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
