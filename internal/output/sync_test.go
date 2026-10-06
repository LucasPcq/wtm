package output

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestConflictFileLines_CapsAtFive(t *testing.T) {
	files := []string{"a", "b", "c", "d", "e", "f", "g"}
	got := conflictFileLines(files)
	want := []string{"a", "b", "c", "d", "e", "…+2 more"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestConflictFileLines_ShortListed(t *testing.T) {
	got := conflictFileLines([]string{"a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %q, want [a b] with no overflow line", got)
	}
}

func TestFormatSyncResult_KeptConflictShowsFooter(t *testing.T) {
	result := domain.SyncResult{
		BaseBranch: "main",
		Steps: []domain.SyncStepResult{
			{
				Branch:         "feat",
				SourceBranch:   "main",
				Path:           "/tmp/wt/feat",
				Status:         domain.SyncStatusConflict,
				ConflictFiles:  []string{"a.go", "b.go"},
				KeptInProgress: true,
			},
		},
	}

	var buf bytes.Buffer
	FormatSyncResult(&buf, result)
	out := buf.String()

	if !strings.Contains(out, "left in progress (2 files)") {
		t.Errorf("expected enriched conflict line, got:\n%s", out)
	}
	if !strings.Contains(out, "Conflicts left in progress") {
		t.Errorf("expected footer header, got:\n%s", out)
	}
	if !strings.Contains(out, "/tmp/wt/feat") {
		t.Errorf("expected worktree path in footer, got:\n%s", out)
	}
	if !strings.Contains(out, "git rebase --continue") {
		t.Errorf("expected resume hint, got:\n%s", out)
	}
	if !strings.Contains(out, "conflicting:") {
		t.Errorf("expected conflicting label, got:\n%s", out)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("expected conflicting files listed vertically, got:\n%s", out)
	}
}

func TestFormatWorktreeState_RebasingPrecedesDirty(t *testing.T) {
	rebasing := formatWorktreeState(domain.WorktreeStatus{IsDirty: true, RebaseInProgress: true})
	if !strings.Contains(rebasing, "rebasing") {
		t.Errorf("expected 'rebasing' badge, got %q", rebasing)
	}
	if strings.Contains(rebasing, "dirty") {
		t.Errorf("rebasing should take precedence over dirty, got %q", rebasing)
	}

	if dirty := formatWorktreeState(domain.WorktreeStatus{IsDirty: true}); !strings.Contains(dirty, "dirty") {
		t.Errorf("expected 'dirty' badge, got %q", dirty)
	}
	if clean := formatWorktreeState(domain.WorktreeStatus{}); !strings.Contains(clean, "clean") {
		t.Errorf("expected 'clean' badge, got %q", clean)
	}
}

func TestFormatSyncResult_AbortedConflictNoFooter(t *testing.T) {
	result := domain.SyncResult{
		BaseBranch: "main",
		Steps: []domain.SyncStepResult{
			{
				Branch:        "feat",
				SourceBranch:  "main",
				Status:        domain.SyncStatusConflict,
				ConflictFiles: []string{"a.go", "b.go"},
			},
		},
	}

	var buf bytes.Buffer
	FormatSyncResult(&buf, result)
	out := buf.String()

	if !strings.Contains(out, "aborted, tree clean (2 files)") {
		t.Errorf("expected enriched aborted line, got:\n%s", out)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("expected conflicting files listed vertically, got:\n%s", out)
	}
	if strings.Contains(out, "Conflicts left in progress") {
		t.Errorf("did not expect footer in aborted mode, got:\n%s", out)
	}
}

func TestFormatSyncResultParentUpdates(t *testing.T) {
	var buf bytes.Buffer
	FormatSyncResult(&buf, domain.SyncResult{
		BaseBranch: "main",
		ParentUpdates: []domain.ParentUpdate{
			{Branch: "feature", Status: domain.ParentFastForwarded, OldTip: "aaa1111", NewTip: "bbb2222"},
			{Branch: "legacy", Status: domain.ParentBehind, Behind: 3, Children: []string{"dev", "other"}},
			{Branch: "split", Status: domain.ParentDiverged},
		},
		Steps: []domain.SyncStepResult{
			{Branch: "dev", SourceBranch: "feature", Status: domain.SyncStatusUpToDate},
		},
	})

	out := buf.String()
	for _, want := range []string{
		"feature fast-forwarded to origin/feature",
		"aaa1111 → bbb2222",
		"legacy is 3 commits behind origin/legacy",
		"dev, other rebased onto it as is",
		"--ff-parents",
		"split has diverged from origin/split",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("recap should contain %q, got:\n%s", want, out)
		}
	}
}

func TestFormatSyncResultNoParentUpdatesPrintsNothingExtra(t *testing.T) {
	var buf bytes.Buffer
	FormatSyncResult(&buf, domain.SyncResult{
		BaseBranch: "main",
		Steps:      []domain.SyncStepResult{{Branch: "dev", Status: domain.SyncStatusUpToDate}},
	})
	if strings.Contains(buf.String(), "behind") || strings.Contains(buf.String(), "fast-forwarded to") {
		t.Errorf("a run with no parent updates should print no parent block, got:\n%s", buf.String())
	}
}

// A cascade where every step rebases onto some other parent never touches the
// base, so the recap may not name it. The plan-header half of this guarantee is
// covered by rules.TestSprintSyncPlanOmitsUntargetedBase.
func TestSyncResultOmitsUntargetedBase(t *testing.T) {
	var buf bytes.Buffer
	FormatSyncResult(&buf, domain.SyncResult{
		BaseBranch: "main",
		Steps:      []domain.SyncStepResult{{Branch: "dev-1", SourceBranch: "feature", Status: domain.SyncStatusUpToDate}},
	})
	if strings.Contains(buf.String(), "Base") || strings.Contains(buf.String(), "main") {
		t.Errorf("recap must not report an untargeted base, got:\n%s", buf.String())
	}
}

func TestFormatSyncResultReportsTargetedBase(t *testing.T) {
	var buf bytes.Buffer
	FormatSyncResult(&buf, domain.SyncResult{
		BaseBranch:   "main",
		BaseTargeted: true,
		BaseUpdated:  true,
		BaseOldTip:   "aaa1111",
		BaseNewTip:   "bbb2222",
		Steps:        []domain.SyncStepResult{{Branch: "feat", SourceBranch: "main", Status: domain.SyncStatusSynced}},
	})
	if !strings.Contains(buf.String(), "main") || !strings.Contains(buf.String(), "aaa1111") {
		t.Errorf("a targeted base should still be reported, got:\n%s", buf.String())
	}
}

// A fast-forward that was asked for and failed must name the obstacle, never
// re-suggest the flag the user already passed — otherwise the same command is
// replayed forever on a stale parent.
func TestFormatSyncResultFailedFastForwardNamesTheObstacle(t *testing.T) {
	var buf bytes.Buffer
	FormatSyncResult(&buf, domain.SyncResult{
		BaseBranch: "main",
		ParentUpdates: []domain.ParentUpdate{{
			Branch:   "feature",
			Status:   domain.ParentFFFailed,
			Detail:   "worktree has uncommitted changes",
			Children: []string{"dev-1"},
		}},
		Steps: []domain.SyncStepResult{{Branch: "dev-1", Status: domain.SyncStatusUpToDate}},
	})

	out := buf.String()
	if !strings.Contains(out, "could not be fast-forwarded to origin/feature") ||
		!strings.Contains(out, "worktree has uncommitted changes") {
		t.Errorf("recap should name the obstacle, got:\n%s", out)
	}
	if strings.Contains(out, "--ff-parents") {
		t.Errorf("must not suggest the flag already passed, got:\n%s", out)
	}
}

// An interrupted sync never concludes "everything is in sync": it names the
// branches it did not reach and says nothing was pushed.
func TestAnInterruptedSyncDoesNotClaimToBeInSync(t *testing.T) {
	steps := []domain.SyncStepResult{
		{Branch: "feat-a", SourceBranch: "main", Status: domain.SyncStatusSynced},
		{Branch: "feat-b", SourceBranch: "feat-a", Status: domain.SyncStatusCancelled},
	}
	var buf bytes.Buffer
	FormatSyncResult(&buf, domain.SyncResult{BaseBranch: "main", Steps: steps})
	FormatSyncPushSummary(&buf, steps)

	got := buf.String()
	if strings.Contains(got, domain.SyncNothingToPush) {
		t.Errorf("output claims the run is in sync:\n%s", got)
	}
	for _, want := range []string{fmt.Sprintf(domain.SyncCancelledLineFmt, "feat-b"), domain.SyncInterruptedSummary} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}
