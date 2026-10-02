package worktreepicker

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestGHBanner(t *testing.T) {
	tests := []struct {
		name      string
		conn      domain.GHConnection
		wantTitle string
	}{
		{name: "ok has no banner", conn: domain.GHConnectionOK, wantTitle: ""},
		{name: "not installed", conn: domain.GHConnectionNotInstalled, wantTitle: "GitHub CLI not found"},
		{name: "not authenticated", conn: domain.GHConnectionNotAuthenticated, wantTitle: "GitHub not connected"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			banner := GHBanner(tc.conn)
			if banner.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", banner.Title, tc.wantTitle)
			}
			if tc.wantTitle == "" {
				if len(banner.Lines) != 0 {
					t.Errorf("expected no lines for OK connection, got %v", banner.Lines)
				}
				return
			}
			if len(banner.Lines) == 0 {
				t.Error("expected body lines for an unavailable connection")
			}
		})
	}
}

func TestBadgesByValue(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "feature/a"},
		{Branch: "feature/b"},
	}
	prs := []domain.PRInfo{{Number: 7, Branch: "feature/b"}}

	badges := BadgesByValue(BadgesByValueParams{Statuses: statuses, PRs: prs})

	// Keyed by the status index as a string, matching SelectItem.Value so
	// SelectListModel.SetBadges can refresh rows without moving the cursor.
	if _, ok := badges["0"]; !ok {
		t.Fatal("expected badges keyed by \"0\"")
	}
	if _, ok := badges["1"]; !ok {
		t.Fatal("expected badges keyed by \"1\"")
	}

	if hasPRBadge(badges["0"]) {
		t.Error("feature/a has no PR, should not get a PR badge")
	}
	if !hasPRBadge(badges["1"]) {
		t.Error("feature/b has PR #7, should get a PR badge")
	}
}

func TestBuildTagsMarksTheActiveWorktree(t *testing.T) {
	active := BuildTags(BuildTagsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/a"},
		ActiveBranch: "feat/a",
	})
	if !hasActiveBadge(active) {
		t.Error("the worktree matching ActiveBranch must carry the active tag")
	}

	other := BuildTags(BuildTagsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/b"},
		ActiveBranch: "feat/a",
	})
	if hasActiveBadge(other) {
		t.Error("a worktree that is not ActiveBranch must not carry the active tag")
	}

	unset := BuildTags(BuildTagsParams{Status: domain.WorktreeStatus{Branch: "feat/a"}})
	if hasActiveBadge(unset) {
		t.Error("an empty ActiveBranch must mark no worktree at all")
	}
}

func TestBuildStatus_RebasingPrecedesDirty(t *testing.T) {
	if pill := BuildStatus(domain.WorktreeStatus{IsDirty: true, RebaseInProgress: true}); pill.Text != "rebasing" {
		t.Fatalf("status pill = %q, want rebasing (should win over dirty)", pill.Text)
	}
	if pill := BuildStatus(domain.WorktreeStatus{IsDirty: true}); pill.Text != "dirty" {
		t.Fatalf("status pill = %q, want dirty", pill.Text)
	}
	if pill := BuildStatus(domain.WorktreeStatus{}); pill.Text != "clean" {
		t.Fatalf("status pill = %q, want clean", pill.Text)
	}
}

func hasPRBadge(badges []components.Badge) bool {
	for _, b := range badges {
		if strings.HasPrefix(b.Text, "PR #") {
			return true
		}
	}
	return false
}

func hasActiveBadge(badges []components.Badge) bool {
	for _, b := range badges {
		if b.Text == domain.WorktreeActiveTag {
			return true
		}
	}
	return false
}

func TestBuildTagsMarksALockedWorktree(t *testing.T) {
	locked := BuildTags(BuildTagsParams{Status: domain.WorktreeStatus{Branch: "feat/a", IsLocked: true}})
	found := false
	for _, tag := range locked {
		if tag.Text == domain.WorktreeLockedTag && tag.Variant == components.BadgeWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("tags = %+v, want a warning tag naming the lock", locked)
	}
	for _, tag := range BuildTags(BuildTagsParams{Status: domain.WorktreeStatus{Branch: "feat/b"}}) {
		if tag.Text == domain.WorktreeLockedTag {
			t.Error("an unlocked worktree must not carry the lock tag")
		}
	}
}
