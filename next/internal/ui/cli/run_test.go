package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
)

func TestTheRefusalOfAStalePlanIsFollowedByTheDryRunThatPlansAgain(t *testing.T) {
	streams, _, _, stderr := output.Test()
	run := Run{Command: "create", Streams: streams, Output: OutputText}

	err := run.Refuse(&domain.Error{Kind: domain.KindPrecondition, Code: domain.CodePlanStale, Message: "the facts changed since the preview"}, "wtm-next create feat/x")

	assert.Error(t, err)
	assert.Contains(t, stderr.String(), "→ wtm-next create feat/x --dry-run")
	assert.Contains(t, stderr.String(), output.ReplanNote)
}

func TestAnyOtherRefusalHasNoDryRunToRunAgain(t *testing.T) {
	streams, _, _, stderr := output.Test()
	run := Run{Command: "create", Streams: streams, Output: OutputText}

	err := run.Refuse(&domain.Error{Kind: domain.KindConflict, Code: "lock.held", Message: "held"}, "wtm-next create feat/x")

	assert.Error(t, err)
	assert.NotContains(t, stderr.String(), "--dry-run")
}
