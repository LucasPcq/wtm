package rules_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func relocateRecapPlan() domain.RelocatePlan {
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

func TestRelocateRecapGroupsAndResolvesParents(t *testing.T) {
	parents := map[string]string{"hotfix": "feature/api"} // legacy falls back to step.Parent

	recap := rules.RelocateRecap(rules.RelocateRecapParams{Plan: relocateRecapPlan(), Parents: parents})

	for _, want := range []string{
		"To apply:",
		"hotfix → ../.trees/hotfix (adopt, parent: feature/api)",
		"legacy  adopt in place (parent: main)",
		"Skipped:",
		"experiment — uncommitted changes",
		"Blocked:",
		"conflicted — target path occupied",
		"serving — jobs are running in it: run `wtm run down serving` first",
		"feat.x shares its name with feat/x (feat-x)",
	} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap missing %q in:\n%s", want, recap)
		}
	}
	if strings.Contains(recap, "base_path:") {
		t.Errorf("no base_path header expected when unchanged, got:\n%s", recap)
	}
}

func TestRelocateRecapShowsBasePathChange(t *testing.T) {
	recap := rules.RelocateRecap(rules.RelocateRecapParams{
		Plan:             relocateRecapPlan(),
		PreviousBasePath: "../old",
	})
	if !strings.Contains(recap, "base_path: ../old → ../.trees") {
		t.Errorf("expected a base_path change header, got:\n%s", recap)
	}
}

func TestRelocateRecapEmpty(t *testing.T) {
	recap := rules.RelocateRecap(rules.RelocateRecapParams{
		Plan: domain.RelocatePlan{BasePath: "../.trees", Steps: []domain.RelocateStep{
			{Branch: "aligned", Status: domain.RelocateStatusNoop},
		}},
	})
	if !strings.Contains(recap, "Nothing to relocate") {
		t.Errorf("expected an empty-state message, got:\n%s", recap)
	}
}

func TestRelocateRecapSetsTheBasePathChangeApartByOneBlankLine(t *testing.T) {
	recap := rules.RelocateRecap(rules.RelocateRecapParams{
		Plan:             relocateRecapPlan(),
		PreviousBasePath: "../old",
	})
	if !strings.HasPrefix(recap, "base_path: ../old → ../.trees\n\nTo apply:") {
		t.Errorf("recap =\n%q", recap)
	}
}
