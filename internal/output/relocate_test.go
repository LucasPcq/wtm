package output

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
		},
	}
}

func TestSprintRelocateRecapGroupsAndResolvesParents(t *testing.T) {
	parents := map[string]string{"hotfix": "feature/api"} // legacy falls back to step.Parent

	recap := SprintRelocateRecap(RelocateRecapParams{Plan: recapPlan(), Parents: parents})

	for _, want := range []string{
		"To apply:",
		"hotfix → ../.trees/hotfix (adopt, parent: feature/api)",
		"legacy  adopt in place (parent: main)",
		"Skipped:",
		"experiment — uncommitted changes",
		"Blocked:",
		"conflicted — target path occupied",
		"serving — jobs are running in it: run `wtm run down serving` first",
	} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap missing %q in:\n%s", want, recap)
		}
	}
	if strings.Contains(recap, "base_path:") {
		t.Errorf("no base_path header expected when unchanged, got:\n%s", recap)
	}
}

func TestSprintRelocateRecapShowsBasePathChange(t *testing.T) {
	recap := SprintRelocateRecap(RelocateRecapParams{
		Plan:             recapPlan(),
		PreviousBasePath: "../old",
	})
	if !strings.Contains(recap, "base_path: ../old → ../.trees") {
		t.Errorf("expected a base_path change header, got:\n%s", recap)
	}
}

func TestSprintRelocateRecapEmpty(t *testing.T) {
	recap := SprintRelocateRecap(RelocateRecapParams{
		Plan: domain.RelocatePlan{BasePath: "../.trees", Steps: []domain.RelocateStep{
			{Branch: "aligned", Status: domain.RelocateStatusNoop},
		}},
	})
	if !strings.Contains(recap, "Nothing to relocate") {
		t.Errorf("expected an empty-state message, got:\n%s", recap)
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
