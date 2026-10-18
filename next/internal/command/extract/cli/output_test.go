package cli

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
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

	formtest.Golden(t, "failed", drawn(outcome))
}

func TestAnUnfinishedTargetSaysTheFilesStayed(t *testing.T) {
	failed := extract.Item{Subject: "feat/x", Status: domain.StatusFailed, Err: &domain.Error{Code: "unit.phase_failed", Message: "on_create failed"}}
	copied := unfinished(failed)
	copied.Plan.Keep = true
	half := unfinished(extract.Item{Subject: "feat/x", Status: domain.StatusFailed, Err: &domain.Error{Code: extract.CodeHalfMoved, Message: "the changes are in \"feat/x\" but \"feat/login\" still holds them"}})

	assert.Contains(t, drawn(unfinished(failed)), "! nothing was moved: the files are still in feat/login\n")
	assert.Contains(t, drawn(copied), "! nothing was copied to feat/x\n")
	assert.NotContains(t, drawn(half), "nothing was")
}

func TestATargetWhoseMoveFailedStillStands(t *testing.T) {
	outcome := unfinished(extract.Item{
		Subject: "feat/clash",
		Status:  domain.StatusFailed,
		Err:     &domain.Error{Kind: domain.KindConflict, Code: extract.CodeConflict, Message: "cannot apply the selected changes"},
		Detail:  &extract.Detail{Path: "/repo.trees/feat-clash"},
	})

	text := drawn(outcome)

	assert.Contains(t, text, "feat/clash  ../repo.trees/feat-clash\n")
	assert.Contains(t, text, "→ wtm go feat/clash")
}

func TestATargetThatNeverStoodHasNoRowNorGo(t *testing.T) {
	outcome := unfinished(extract.Item{Subject: "feat/slow", Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted})

	formtest.Golden(t, "interrupted", drawn(outcome))
}
