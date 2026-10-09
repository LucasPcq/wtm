package components

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestBranchItemsPinsAndGroupsRemotes(t *testing.T) {
	candidates := []domain.BranchCandidate{
		{Name: "alpha"},
		{Name: "main"},
		{Name: "origin/release", IsRemote: true},
	}

	items := BranchItems(BranchItemsParams{
		Candidates:   candidates,
		Pinned:       "main",
		PinnedSuffix: " (default)",
	})

	if len(items) != 4 {
		t.Fatalf("expected pinned + local + separator + remote, got %d: %+v", len(items), items)
	}
	if items[0].Value != "main" || items[0].Label != "main (default)" {
		t.Errorf("expected pinned main first, got %+v", items[0])
	}
	if items[1].Value != "alpha" {
		t.Errorf("expected local alpha second, got %+v", items[1])
	}
	if !items[2].Separator {
		t.Errorf("expected separator before remotes, got %+v", items[2])
	}
	if items[3].Value != "origin/release" || len(items[3].Badges) == 0 || items[3].Badges[0].Text != domain.BadgeTextRemote {
		t.Errorf("expected remote item tagged remote, got %+v", items[3])
	}
}

func TestBranchItemsAddsMissingPinned(t *testing.T) {
	items := BranchItems(BranchItemsParams{
		Candidates:   []domain.BranchCandidate{{Name: "dev"}},
		Pinned:       "main",
		PinnedSuffix: " (detected)",
	})

	if len(items) != 2 || items[0].Value != "main" {
		t.Fatalf("expected detected default pinned first even when absent, got %+v", items)
	}
}

func TestBranchItemsExcludesValue(t *testing.T) {
	items := BranchItems(BranchItemsParams{
		Candidates: []domain.BranchCandidate{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Exclude:    "b",
	})

	for _, it := range items {
		if it.Value == "b" {
			t.Fatalf("excluded value must not appear, got %+v", items)
		}
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items after exclusion, got %d", len(items))
	}
}

func TestBranchItemsTagsDivergence(t *testing.T) {
	candidates := []domain.BranchCandidate{
		{Name: "behind", Behind: 5, State: domain.DivergenceBehind},
		{Name: "ahead", Ahead: 2, State: domain.DivergenceAhead},
		{Name: "diverged", Ahead: 2, Behind: 5, State: domain.DivergenceDiverged},
		{Name: "clean", State: domain.DivergenceUpToDate},
		{Name: "local-only", State: domain.DivergenceUnknown},
	}

	items := BranchItems(BranchItemsParams{Candidates: candidates})

	byValue := make(map[string]SelectItem, len(items))
	for _, it := range items {
		byValue[it.Value] = it
	}

	cases := []struct {
		value   string
		want    string
		variant BadgeVariant
	}{
		{"behind", "↓5", BadgeWarning},
		{"ahead", "↑2", BadgeNeutral},
		{"diverged", "↑2 ↓5", BadgeDanger},
	}
	for _, c := range cases {
		it := byValue[c.value]
		if len(it.Badges) == 0 {
			t.Fatalf("%s: expected a divergence badge, got none", c.value)
		}
		badge := it.Badges[len(it.Badges)-1]
		if badge.Text != c.want || badge.Variant != c.variant {
			t.Errorf("%s: badge = %+v, want text %q variant %v", c.value, badge, c.want, c.variant)
		}
	}

	for _, value := range []string{"clean", "local-only"} {
		if len(byValue[value].Badges) != 0 {
			t.Errorf("%s: expected no divergence badge, got %+v", value, byValue[value].Badges)
		}
	}
}

func TestBranchItemsNoSeparatorWithoutRemotes(t *testing.T) {
	items := BranchItems(BranchItemsParams{
		Candidates: []domain.BranchCandidate{{Name: "a"}, {Name: "b"}},
	})

	for _, it := range items {
		if it.Separator {
			t.Fatalf("no separator expected without remotes, got %+v", items)
		}
	}
}
