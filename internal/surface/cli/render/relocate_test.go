package render

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func recapPlan() domain.RelocatePlan {
	return domain.RelocatePlan{
		BasePath: "../.trees",
		Steps: []domain.RelocateStep{
			{Branch: "hotfix", ToPath: "/repo/../.trees/hotfix", Status: domain.RelocateStatusMove, Adopt: true, Parent: "main"},
			{Branch: "legacy", ToPath: "/repo/../.trees/legacy", Status: domain.RelocateStatusAdopt, Adopt: true, Parent: "main"},
			{Branch: "experiment", Status: domain.RelocateStatusSkippedDirty},
			{Branch: "conflicted", Status: domain.RelocateStatusBlockedDest},
			{Branch: "serving", Status: domain.RelocateStatusBlockedJobs},
			{Branch: "feat.x", Status: domain.RelocateStatusBlockedName, Detail: "feat.x shares its name with feat/x (feat-x) — rename one of the two branches to adopt it"},
		},
	}
}

func TestFormatRelocateNamesTheWayOutOfRunningJobs(t *testing.T) {
	var plan strings.Builder
	FormatRelocatePlan(&plan, recapPlan())
	var result strings.Builder
	FormatRelocateResult(&result, domain.RelocateResult{BasePath: "../.trees", Steps: []domain.RelocateStepResult{
		{Branch: "serving", Status: domain.RelocateStatusBlockedJobs},
	}})
	for name, out := range map[string]string{"plan": plan.String(), "result": result.String()} {
		if !strings.Contains(out, "wtm run down serving") {
			t.Errorf("%s does not name `wtm run down serving`:\n%s", name, out)
		}
	}
}
