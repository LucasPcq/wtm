package cli

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
)

func TestAnOutcomeWithoutItsTargetDrawsNothing(t *testing.T) {
	var out bytes.Buffer

	drawOutcome(&out, extract.Outcome{Result: &extract.Result{}}, "")

	assert.Empty(t, out.String())
}

func TestATargetLeftIncompleteStandsAndSaysWhy(t *testing.T) {
	outcome := unfinished(extract.Item{
		Subject: "feat/bad",
		Status:  domain.StatusFailed,
		Reason:  domain.ReasonIncomplete,
		Err:     &domain.Error{Kind: domain.KindInternal, Code: "unit.phase_failed", Message: "on_create failed: hook failed: exit status 3"},
		Detail:  &extract.Detail{Path: "/repo.trees/feat-bad"},
	})

	golden(t, "failed", drawn(outcome))
}

func TestATargetThatNeverStoodHasNoRowNorGo(t *testing.T) {
	outcome := unfinished(extract.Item{Subject: "feat/slow", Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted})

	golden(t, "interrupted", drawn(outcome))
}

// unfinished is the outcome of moving README.md to a new target, ended as item.
func unfinished(item extract.Item) extract.Outcome {
	plan := extract.Plan{
		Source: "feat/login",
		Files:  []worktree.Change{{Path: "README.md", Status: worktree.ChangeModified}},
		Target: extract.Target{Branch: item.Subject, Action: extract.ActionCreate},
		From:   "main",
	}
	return extract.Outcome{Plan: plan, Result: &extract.Result{Items: []extract.Item{item}}}
}

func drawn(outcome extract.Outcome) string {
	var out bytes.Buffer
	drawOutcome(&out, outcome, "/repo")
	return out.String()
}
