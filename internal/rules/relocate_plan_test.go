package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	testProjectDir = "/repo"
	testBasePath   = "../.trees"
	testBaseBranch = "main"
)

func desired(branch string) string {
	return filepath.Join(testProjectDir, testBasePath, rules.SanitizeBranchName(branch))
}

func planFor(t *testing.T, force bool, candidates ...rules.RelocateCandidate) domain.RelocatePlan {
	t.Helper()
	return rules.BuildRelocatePlan(rules.BuildRelocatePlanParams{
		Candidates: candidates,
		ProjectDir: testProjectDir,
		BasePath:   testBasePath,
		BaseBranch: testBaseBranch,
		Force:      force,
	})
}

func stepFor(t *testing.T, plan domain.RelocatePlan, branch string) domain.RelocateStep {
	t.Helper()
	for _, s := range plan.Steps {
		if s.Branch == branch {
			return s
		}
	}
	t.Fatalf("no step for branch %q in plan %+v", branch, plan.Steps)
	return domain.RelocateStep{}
}

func TestBuildRelocatePlanExcludesMain(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:   "main",
		FromPath: testProjectDir,
		IsMain:   true,
	})
	if len(plan.Steps) != 0 {
		t.Fatalf("expected main to be excluded, got %+v", plan.Steps)
	}
}

func TestBuildRelocatePlanManagedInPlaceIsNoop(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:    "feat/x",
		FromPath:  desired("feat/x"),
		IsManaged: true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusNoop {
		t.Fatalf("expected noop, got %q", step.Status)
	}
	if step.Adopt {
		t.Fatalf("managed worktree should not be adopted")
	}
}

func TestBuildRelocatePlanManagedNeedsMove(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:    "feat/x",
		FromPath:  "/somewhere/else",
		IsManaged: true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusMove {
		t.Fatalf("expected move, got %q", step.Status)
	}
	if step.Adopt {
		t.Fatalf("managed worktree should not be adopted")
	}
	if step.ToPath != desired("feat/x") {
		t.Fatalf("unexpected ToPath %q", step.ToPath)
	}
}

func TestBuildRelocatePlanExternalNeedsMoveIsAdopted(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:    "feat/y",
		FromPath:  "/manual/feat-y",
		IsManaged: false,
	})
	step := stepFor(t, plan, "feat/y")
	if step.Status != domain.RelocateStatusMove {
		t.Fatalf("expected move, got %q", step.Status)
	}
	if !step.Adopt {
		t.Fatalf("external worktree should be adopted on move")
	}
	if step.Parent != testBaseBranch {
		t.Fatalf("expected default parent %q, got %q", testBaseBranch, step.Parent)
	}
}

func TestBuildRelocatePlanExternalInPlaceIsAdoptOnly(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:    "feat/z",
		FromPath:  desired("feat/z"),
		IsManaged: false,
	})
	step := stepFor(t, plan, "feat/z")
	if step.Status != domain.RelocateStatusAdopt {
		t.Fatalf("expected adopt, got %q", step.Status)
	}
	if !step.Adopt {
		t.Fatalf("external worktree should be adopted")
	}
}

func TestBuildRelocatePlanDirtyMoveSkippedWithoutForce(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:    "feat/x",
		FromPath:  "/somewhere/else",
		IsManaged: true,
		IsDirty:   true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusSkippedDirty {
		t.Fatalf("expected skipped_dirty, got %q", step.Status)
	}
}

func TestBuildRelocatePlanDirtyMoveProceedsWithForce(t *testing.T) {
	plan := planFor(t, true, rules.RelocateCandidate{
		Branch:    "feat/x",
		FromPath:  "/somewhere/else",
		IsManaged: true,
		IsDirty:   true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusMove {
		t.Fatalf("expected move with force, got %q", step.Status)
	}
}

func TestBuildRelocatePlanLockedMoveSkippedWithoutForce(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:    "feat/x",
		FromPath:  "/somewhere/else",
		IsManaged: true,
		IsLocked:  true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusSkippedLocked {
		t.Fatalf("expected skipped_locked, got %q", step.Status)
	}
}

func TestBuildRelocatePlanDestOccupiedIsBlockedEvenWithForce(t *testing.T) {
	plan := planFor(t, true, rules.RelocateCandidate{
		Branch:       "feat/x",
		FromPath:     "/somewhere/else",
		IsManaged:    true,
		DestOccupied: true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusBlockedDest {
		t.Fatalf("expected blocked_dest, got %q", step.Status)
	}
}

func TestPlanHasWork(t *testing.T) {
	noop := planFor(t, false, rules.RelocateCandidate{
		Branch: "feat/x", FromPath: desired("feat/x"), IsManaged: true,
	})
	if rules.PlanHasWork(noop) {
		t.Fatalf("expected no work for an all-noop plan")
	}

	work := planFor(t, false, rules.RelocateCandidate{
		Branch: "feat/x", FromPath: "/elsewhere", IsManaged: true,
	})
	if !rules.PlanHasWork(work) {
		t.Fatalf("expected work for a plan with a move")
	}
}

func TestPlanAdoptionsReturnsOnlyAdoptedActionableSteps(t *testing.T) {
	plan := planFor(t, false,
		rules.RelocateCandidate{Branch: "ext-move", FromPath: "/elsewhere", IsManaged: false},
		rules.RelocateCandidate{Branch: "ext-inplace", FromPath: desired("ext-inplace"), IsManaged: false},
		rules.RelocateCandidate{Branch: "managed-move", FromPath: "/elsewhere2", IsManaged: true},
		rules.RelocateCandidate{Branch: "ext-blocked", FromPath: "/elsewhere3", IsManaged: false, DestOccupied: true},
	)
	adoptions := rules.PlanAdoptions(plan)
	if len(adoptions) != 2 {
		t.Fatalf("expected 2 adoptions (ext-move, ext-inplace), got %d: %+v", len(adoptions), adoptions)
	}
	for _, s := range adoptions {
		if s.Branch == "managed-move" || s.Branch == "ext-blocked" {
			t.Fatalf("unexpected adoption candidate %q", s.Branch)
		}
	}
}

func TestRelocateHasFailure(t *testing.T) {
	clean := domain.RelocateResult{Steps: []domain.RelocateStepResult{
		{Branch: "a", Status: domain.RelocateStatusMoved},
		{Branch: "b", Status: domain.RelocateStatusSkippedDirty},
	}}
	if rules.RelocateHasFailure(clean) {
		t.Fatalf("skips are not failures")
	}

	withError := domain.RelocateResult{Steps: []domain.RelocateStepResult{
		{Branch: "a", Status: domain.RelocateStatusError},
	}}
	if !rules.RelocateHasFailure(withError) {
		t.Fatalf("error must be a failure")
	}

	withBlocked := domain.RelocateResult{Steps: []domain.RelocateStepResult{
		{Branch: "a", Status: domain.RelocateStatusBlockedDest},
	}}
	if !rules.RelocateHasFailure(withBlocked) {
		t.Fatalf("blocked_dest must be a failure")
	}
}

func TestBuildRelocatePlanInspectErrMoveIsError(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:     "feat/x",
		FromPath:   "/somewhere/else",
		IsManaged:  true,
		InspectErr: true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusError {
		t.Fatalf("expected error when a moving worktree cannot be inspected, got %q", step.Status)
	}
}

func TestBuildRelocatePlanInspectErrMoveProceedsWithForce(t *testing.T) {
	plan := planFor(t, true, rules.RelocateCandidate{
		Branch:     "feat/x",
		FromPath:   "/somewhere/else",
		IsManaged:  true,
		InspectErr: true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusMove {
		t.Fatalf("expected move with force despite inspect error, got %q", step.Status)
	}
}

func TestBuildRelocatePlanInspectErrInPlaceIsNotError(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:     "feat/x",
		FromPath:   desired("feat/x"),
		IsManaged:  true,
		InspectErr: true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusNoop {
		t.Fatalf("an in-place worktree needs no inspection, expected noop, got %q", step.Status)
	}
}

func TestBuildRelocatePlanExternalInPlaceDirtyStillAdopts(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:    "feat/z",
		FromPath:  desired("feat/z"),
		IsManaged: false,
		IsDirty:   true,
	})
	step := stepFor(t, plan, "feat/z")
	if step.Status != domain.RelocateStatusAdopt {
		t.Fatalf("adopt-in-place should not be blocked by dirty, got %q", step.Status)
	}
}

func TestReprojectRelocatePlanRetargetsAndReclassifies(t *testing.T) {
	// An aligned (noop) managed worktree and an adopt-in-place external, both under
	// the current base_path, re-projected onto a new base_path.
	aligned := desired("feat")
	external := desired("ext")
	plan := domain.RelocatePlan{
		BasePath:   testBasePath,
		BaseBranch: testBaseBranch,
		Steps: []domain.RelocateStep{
			{Branch: "feat", FromPath: aligned, ToPath: aligned, Status: domain.RelocateStatusNoop},
			{Branch: "ext", FromPath: external, ToPath: external, Status: domain.RelocateStatusAdopt, Adopt: true, Parent: "main"},
		},
	}

	got := rules.ReprojectRelocatePlan(rules.ReprojectRelocatePlanParams{
		Plan:       plan,
		ProjectDir: testProjectDir,
		BasePath:   "../elsewhere",
	})

	if got.BasePath != "../elsewhere" {
		t.Fatalf("base path = %q, want ../elsewhere", got.BasePath)
	}
	feat := stepFor(t, got, "feat")
	if feat.Status != domain.RelocateStatusMove {
		t.Errorf("aligned worktree should become a move under the new base_path, got %v", feat.Status)
	}
	if want := filepath.Join(testProjectDir, "../elsewhere", "feat"); feat.ToPath != want {
		t.Errorf("feat target = %q, want %q", feat.ToPath, want)
	}
	ext := stepFor(t, got, "ext")
	if ext.Status != domain.RelocateStatusMove || !ext.Adopt {
		t.Errorf("external should move + stay adopted, got status=%v adopt=%v", ext.Status, ext.Adopt)
	}
}

// A worktree whose jobs are running is blocked, --force or not: they are keyed
// on its path, and the way out is to stop them, not to move them blind.
func TestBuildRelocatePlanRunningJobsBlockEvenWithForce(t *testing.T) {
	plan := planFor(t, true, rules.RelocateCandidate{
		Branch:    "feat/x",
		FromPath:  "/somewhere/else",
		IsManaged: true,
		HasJobs:   true,
	})
	step := stepFor(t, plan, "feat/x")
	if step.Status != domain.RelocateStatusBlockedJobs {
		t.Fatalf("expected blocked_jobs, got %q", step.Status)
	}
	if !rules.RelocateHasFailure(domain.RelocateResult{Steps: []domain.RelocateStepResult{{Status: step.Status}}}) {
		t.Error("a move refused for running jobs must fail the run")
	}
}

func TestBuildRelocatePlanRunningJobsDoNotBlockAnAdoptionInPlace(t *testing.T) {
	plan := planFor(t, false, rules.RelocateCandidate{
		Branch:   "feat/x",
		FromPath: desired("feat/x"),
		HasJobs:  true,
	})
	if step := stepFor(t, plan, "feat/x"); step.Status != domain.RelocateStatusAdopt {
		t.Fatalf("expected adopt, got %q", step.Status)
	}
}
