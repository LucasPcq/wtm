package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSyncIncludesBase(t *testing.T) {
	tests := []struct {
		name     string
		selected []string
		base     string
		want     bool
	}{
		{"nil selection means all", nil, "main", true},
		{"base explicitly selected", []string{"feat", "main"}, "main", true},
		{"base not selected", []string{"feat"}, "main", false},
		{"empty non-nil selection", []string{}, "main", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SyncIncludesBase(SyncIncludesBaseParams{Selected: tt.selected, BaseBranch: tt.base})
			if got != tt.want {
				t.Fatalf("SyncIncludesBase(%v, %q) = %v, want %v", tt.selected, tt.base, got, tt.want)
			}
		})
	}
}

func TestPushableCount(t *testing.T) {
	steps := []domain.SyncStepResult{
		{PushPending: true, Pushed: false},  // counts
		{PushPending: true, Pushed: true},   // already pushed
		{PushPending: false, Pushed: false}, // nothing to push
		{PushPending: true, Pushed: false},  // counts
	}
	if got := PushableCount(steps); got != 2 {
		t.Fatalf("PushableCount = %d, want 2", got)
	}
	if got := PushableCount(nil); got != 0 {
		t.Fatalf("PushableCount(nil) = %d, want 0", got)
	}
}

func TestHasSyncFailure(t *testing.T) {
	tests := []struct {
		name  string
		steps []domain.SyncStepResult
		want  bool
	}{
		{"empty", nil, false},
		{"all ok", []domain.SyncStepResult{{Status: domain.SyncStatusSynced}}, false},
		{"conflict", []domain.SyncStepResult{{Status: domain.SyncStatusConflict}}, true},
		{"error", []domain.SyncStepResult{{Status: domain.SyncStatusError}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasSyncFailure(tt.steps); got != tt.want {
				t.Fatalf("HasSyncFailure = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecidePush(t *testing.T) {
	tests := []struct {
		name   string
		params DecidePushParams
		want   PushDecision
	}{
		{"nothing pushable", DecidePushParams{Push: true, PushableCount: 0}, PushSkip},
		{"no-push wins", DecidePushParams{Push: true, NoPush: true, PushableCount: 2}, PushSkip},
		{"force with push flag", DecidePushParams{Push: true, PushableCount: 2}, PushForce},
		{"non-interactive without push", DecidePushParams{Interactive: false, PushableCount: 2}, PushSkip},
		{"interactive asks", DecidePushParams{Interactive: true, PushableCount: 2}, PushConfirm},
		{"yes without push skips", DecidePushParams{Yes: true, Interactive: true, PushableCount: 2}, PushSkip},
		{"yes with push forces", DecidePushParams{Yes: true, Push: true, Interactive: true, PushableCount: 2}, PushForce},
		{"yes with no-push skips", DecidePushParams{Yes: true, NoPush: true, Interactive: true, PushableCount: 2}, PushSkip},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecidePush(tt.params); got != tt.want {
				t.Fatalf("DecidePush = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommitCountLabel(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{n: 0, want: "0 commits"},
		{n: 1, want: "1 commit"},
		{n: 2, want: "2 commits"},
	}
	for _, tc := range tests {
		if got := CommitCountLabel(tc.n); got != tc.want {
			t.Errorf("CommitCountLabel(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestBaseIsTarget(t *testing.T) {
	offBase := []domain.SyncStep{
		{Branch: "dev-1", SourceBranch: "feature"},
		{Branch: "dev-2", SourceBranch: "feature"},
	}
	onBase := []domain.SyncStep{{Branch: "feat", SourceBranch: "main"}}

	tests := []struct {
		name string
		in   BaseIsTargetParams
		want bool
	}{
		{
			name: "a step rebases onto the base",
			in:   BaseIsTargetParams{Steps: onBase, Selected: []string{"feat"}, BaseBranch: "main"},
			want: true,
		},
		{
			// The reported bug: an explicit selection whose every step hangs off
			// another parent must leave the base completely alone.
			name: "explicit selection, nothing targets the base",
			in:   BaseIsTargetParams{Steps: offBase, Selected: []string{"dev-1", "dev-2"}, BaseBranch: "main"},
			want: false,
		},
		{
			// --all covers the whole forest including its root, even when no
			// worktree happens to hang off the base.
			name: "--all with nothing targeting the base",
			in:   BaseIsTargetParams{Steps: offBase, Selected: nil, BaseBranch: "main"},
			want: true,
		},
		{
			name: "the selection names the base",
			in:   BaseIsTargetParams{Steps: offBase, Selected: []string{"dev-1", "main"}, BaseBranch: "main"},
			want: true,
		},
		{
			name: "no step at all is the base-only refresh",
			in:   BaseIsTargetParams{Selected: nil, BaseBranch: "main"},
			want: true,
		},
		{
			name: "no step and a selection that ignores the base",
			in:   BaseIsTargetParams{Selected: []string{"dev-1"}, BaseBranch: "main"},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BaseIsTarget(tc.in); got != tc.want {
				t.Fatalf("BaseIsTarget(%+v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSyncSelectsOnlyBase(t *testing.T) {
	tests := []struct {
		name     string
		selected []string
		want     bool
	}{
		{name: "every worktree", selected: nil, want: false},
		{name: "the base alone", selected: []string{"main"}, want: true},
		{name: "the base and a worktree", selected: []string{"main", "feat-a"}, want: false},
		{name: "worktrees only", selected: []string{"feat-a"}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SyncSelectsOnlyBase(SyncIncludesBaseParams{Selected: tc.selected, BaseBranch: "main"})
			if got != tc.want {
				t.Fatalf("SyncSelectsOnlyBase(%v) = %v, want %v", tc.selected, got, tc.want)
			}
		})
	}
}

func TestSyncStatusLabelNamesEveryOutcome(t *testing.T) {
	tests := []struct {
		status domain.SyncStepStatus
		want   string
	}{
		{domain.SyncStatusSynced, domain.SyncLabelSynced},
		{domain.SyncStatusUpToDate, domain.SyncLabelUpToDate},
		{domain.SyncStatusSkippedDirty, domain.SyncLabelSkippedDirty},
		{domain.SyncStatusSkippedAncestor, domain.SyncLabelSkippedAncestor},
		{domain.SyncStatusDiverged, domain.SyncLabelDiverged},
		{domain.SyncStatusRebaseInProgress, domain.SyncLabelRebaseInProgress},
		{domain.SyncStatusConflict, domain.SyncLabelConflict},
		{domain.SyncStatusUnknownParent, domain.SyncLabelUnknownParent},
		{domain.SyncStatusError, domain.SyncLabelError},
	}

	for _, tc := range tests {
		if got := SyncStatusLabel(tc.status); got != tc.want {
			t.Errorf("SyncStatusLabel(%s) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// An unknown status must still read as something: a blank line in a run recap
// hides a step that did happen.
func TestSyncStatusLabelFallsBackToTheStatusItself(t *testing.T) {
	if got := SyncStatusLabel(domain.SyncStepStatus("weather")); got != "weather" {
		t.Errorf("SyncStatusLabel(weather) = %q, want the status itself", got)
	}
}

func TestSyncBaseAndParentLabels(t *testing.T) {
	if got := SyncBaseLabel(true); got != domain.SyncBaseLabelFastForwarded {
		t.Errorf("SyncBaseLabel(true) = %q", got)
	}
	if got := SyncBaseLabel(false); got != domain.SyncBaseLabelUpToDate {
		t.Errorf("SyncBaseLabel(false) = %q", got)
	}
	if got := SyncParentStatusLabel(domain.ParentDiverged); got != domain.SyncParentLabelDiverged {
		t.Errorf("SyncParentStatusLabel(diverged) = %q", got)
	}
}

// --all is an answer, not an offer: it covers every worktree, including the ones
// the cascade will skip, so the run reports why each was skipped instead of
// silently leaving them out. Only the base is excluded — it is fast-forwarded,
// never rebased.
func TestSyncAllBranchesCoverEveryWorktreeButTheBase(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", IsParent: true},
		{Branch: "clean"},
		{Branch: "dirty", IsDirty: true},
		{Branch: "stuck", RebaseInProgress: true},
	}

	got := SyncAllBranches(statuses)

	if strings.Join(got, ",") != "clean,dirty,stuck" {
		t.Errorf("SyncAllBranches = %v, want [clean dirty stuck]", got)
	}
}

// A repository whose only worktree is the base has nothing to rebase, and says
// so with an empty list rather than with nil, which the sync flow reads as
// "the selection was never made".
func TestSyncAllBranchesIsEmptyWhenOnlyTheBaseExists(t *testing.T) {
	got := SyncAllBranches([]domain.WorktreeStatus{{Branch: "main", IsParent: true}})

	if got == nil || len(got) != 0 {
		t.Errorf("SyncAllBranches = %#v, want an empty list, never nil", got)
	}
}

// A surface offering "sync everything" pre-checks what the run can act on: the
// base is in — it gets fast-forwarded, and every chain below it wants it fresh —
// while what the cascade would skip stays listed and unchecked rather than
// silently dropped.
func TestSyncReadyBranchesKeepTheBaseAndDropWhatWouldBeSkipped(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", IsParent: true},
		{Branch: "clean"},
		{Branch: "dirty", IsDirty: true},
		{Branch: "stuck", RebaseInProgress: true},
	}

	got := SyncReadyBranches(statuses)

	if len(got) != 2 || got[0] != "main" || got[1] != "clean" {
		t.Errorf("SyncReadyBranches = %v, want [main clean]", got)
	}
}

func TestWorktreeNodesPairsEachStatusWithItsRecordedParent(t *testing.T) {
	nodes := WorktreeNodes(WorktreeNodesParams{
		Statuses: []domain.WorktreeStatus{{Branch: "main", IsParent: true}, {Branch: "a", Path: "/wt/a"}},
		Parents:  map[string]string{"a": "main"},
	})

	if len(nodes) != 2 {
		t.Fatalf("nodes = %+v, want one per status", nodes)
	}
	if !nodes[0].IsMain || nodes[0].SourceBranch != "" {
		t.Errorf("root = %+v, want the base carrying no parent", nodes[0])
	}
	if nodes[1].SourceBranch != "main" || nodes[1].Path != "/wt/a" {
		t.Errorf("node = %+v, want the recorded parent and path carried over", nodes[1])
	}
}

// "failed" alone sends the user looking for a cause the run already knows.
func TestSyncStepLabelCarriesTheCauseOfAFailure(t *testing.T) {
	label := SyncStepLabel(domain.SyncStepResult{
		Status: domain.SyncStatusError,
		Detail: "could not read HEAD",
	})

	if !strings.Contains(label, "could not read HEAD") {
		t.Errorf("SyncStepLabel = %q, want the cause named", label)
	}
}

func TestSyncStepLabelFallsBackWhenAFailureCarriesNoCause(t *testing.T) {
	if got := SyncStepLabel(domain.SyncStepResult{Status: domain.SyncStatusError}); got != domain.SyncLabelError {
		t.Errorf("SyncStepLabel = %q, want the bare failure label", got)
	}
}

// The two conflict modes leave the worktree in opposite states: one has a rebase
// to finish, the other has nothing to clean up.
func TestSyncStepLabelTellsTheTwoConflictModesApart(t *testing.T) {
	kept := SyncStepLabel(domain.SyncStepResult{Status: domain.SyncStatusConflict, KeptInProgress: true})
	aborted := SyncStepLabel(domain.SyncStepResult{Status: domain.SyncStatusConflict})

	if kept == aborted {
		t.Fatalf("both conflict modes read %q, want them told apart", kept)
	}
	if got := SyncStepLabel(domain.SyncStepResult{Status: domain.SyncStatusSynced}); got != domain.SyncLabelSynced {
		t.Errorf("SyncStepLabel(synced) = %q, want the plain status label", got)
	}
}
