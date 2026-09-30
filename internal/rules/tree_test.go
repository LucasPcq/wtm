package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func at(day int) time.Time {
	return time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)
}

func findRoot(forest domain.Forest, branch string) (domain.TreeNode, bool) {
	for _, r := range forest.Roots {
		if r.Branch == branch {
			return r, true
		}
	}
	return domain.TreeNode{}, false
}

func childBranches(node domain.TreeNode) []string {
	out := make([]string, 0, len(node.Children))
	for _, c := range node.Children {
		out = append(out, c.Branch)
	}
	return out
}

func TestBuildForestSimpleStack(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "feat", SourceBranch: "main", CreatedAt: at(2)},
			{Branch: "feat-ui", SourceBranch: "feat", CreatedAt: at(3)},
		},
	})

	if len(forest.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d: %+v", len(forest.Roots), forest.Roots)
	}
	main := forest.Roots[0]
	if main.Branch != "main" || !main.IsMain {
		t.Fatalf("expected main root, got %+v", main)
	}
	if got := childBranches(main); len(got) != 1 || got[0] != "feat" {
		t.Fatalf("expected [feat] under main, got %v", got)
	}
	if got := childBranches(main.Children[0]); len(got) != 1 || got[0] != "feat-ui" {
		t.Fatalf("expected [feat-ui] under feat, got %v", got)
	}
}

func TestBuildForestVirtualRoot(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "spike", SourceBranch: "dev", CreatedAt: at(2)},
		},
	})

	dev, ok := findRoot(forest, "dev")
	if !ok {
		t.Fatalf("expected virtual root 'dev', got roots %+v", forest.Roots)
	}
	if !dev.IsVirtual || dev.Path != "" {
		t.Fatalf("expected virtual dev root with no path, got %+v", dev)
	}
	if got := childBranches(dev); len(got) != 1 || got[0] != "spike" {
		t.Fatalf("expected [spike] under dev, got %v", got)
	}
}

func TestBuildForestMainFirstThenVirtual(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "spike", SourceBranch: "dev", CreatedAt: at(2)},
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
		},
	})

	if len(forest.Roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(forest.Roots))
	}
	if !forest.Roots[0].IsMain {
		t.Fatalf("expected main first, got %+v", forest.Roots[0])
	}
	if !forest.Roots[1].IsVirtual {
		t.Fatalf("expected virtual root last, got %+v", forest.Roots[1])
	}
}

func TestBuildForestSiblingsByCreatedAt(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "younger", SourceBranch: "main", CreatedAt: at(5)},
			{Branch: "older", SourceBranch: "main", CreatedAt: at(2)},
		},
	})

	got := childBranches(forest.Roots[0])
	if len(got) != 2 || got[0] != "older" || got[1] != "younger" {
		t.Fatalf("expected [older younger] by creation date, got %v", got)
	}
}

func TestBuildForestCycleDoesNotPanic(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "a", SourceBranch: "b", CreatedAt: at(2)},
			{Branch: "b", SourceBranch: "a", CreatedAt: at(3)},
		},
	})

	cycleFlagged := false
	var walk func(n domain.TreeNode)
	walk = func(n domain.TreeNode) {
		if n.Status.InCycle {
			cycleFlagged = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range forest.Roots {
		walk(r)
	}

	if !cycleFlagged {
		t.Fatalf("expected at least one node flagged InCycle, got %+v", forest.Roots)
	}
}

func TestBuildForestStatusPreserved(t *testing.T) {
	forest := BuildForest(BuildForestParams{
		Nodes: []ForestNode{
			{Branch: "main", IsMain: true, CreatedAt: at(1)},
			{Branch: "feat", SourceBranch: "main", CreatedAt: at(2), Status: domain.TreeNodeStatus{
				CommitsAhead: 3,
				IsDirty:      true,
				NeedsSync:    true,
			}},
		},
	})

	feat := forest.Roots[0].Children[0]
	if feat.Status.CommitsAhead != 3 || !feat.Status.IsDirty || !feat.Status.NeedsSync {
		t.Fatalf("status not preserved: %+v", feat.Status)
	}
}

// flattenSample mirrors the forest output/tree_test.go renders, so the shape the
// two surfaces share is pinned where it is now decided.
func flattenSample() domain.Forest {
	return domain.Forest{
		Roots: []domain.TreeNode{
			{
				Branch: "main", IsMain: true,
				Children: []domain.TreeNode{
					{
						Branch: "feat",
						Children: []domain.TreeNode{
							{Branch: "feat-ui"},
							{Branch: "feat-api"},
						},
					},
					{Branch: "billing"},
				},
			},
			{Branch: "dev", IsVirtual: true},
		},
	}
}

func TestFlattenForestOrdersDepthFirst(t *testing.T) {
	rows := FlattenForest(flattenSample())

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Node.Branch)
	}
	want := []string{"main", "feat", "feat-ui", "feat-api", "billing", "dev"}
	if len(got) != len(want) {
		t.Fatalf("got %d rows %v, want %d", len(got), got, len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("row %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestFlattenForestPrefixes(t *testing.T) {
	prefixes := make(map[string]string)
	depths := make(map[string]int)
	for _, row := range FlattenForest(flattenSample()) {
		prefixes[row.Node.Branch] = row.Prefix
		depths[row.Node.Branch] = row.Depth
	}

	cases := []struct {
		branch string
		prefix string
		depth  int
	}{
		{"main", "", 0},
		{"feat", domain.TreeConnectorBranch, 1},
		// A non-last parent keeps the pipe running down its children's gutter.
		{"feat-ui", domain.TreeGutterPipe + domain.TreeConnectorBranch, 2},
		{"feat-api", domain.TreeGutterPipe + domain.TreeConnectorLast, 2},
		{"billing", domain.TreeConnectorLast, 1},
		{"dev", "", 0},
	}
	for _, tc := range cases {
		if prefixes[tc.branch] != tc.prefix {
			t.Errorf("%s prefix = %q, want %q", tc.branch, prefixes[tc.branch], tc.prefix)
		}
		if depths[tc.branch] != tc.depth {
			t.Errorf("%s depth = %d, want %d", tc.branch, depths[tc.branch], tc.depth)
		}
	}
}

func TestFlattenForestEmpty(t *testing.T) {
	if rows := FlattenForest(domain.Forest{}); len(rows) != 0 {
		t.Errorf("an empty forest should flatten to no rows, got %d", len(rows))
	}
}

func TestTreeBadgesFollowTheCanonicalOrder(t *testing.T) {
	node := domain.TreeNode{
		IsVirtual: true,
		Status: domain.TreeNodeStatus{
			PR:           &domain.WorktreeListPR{Number: 7},
			CommitsAhead: 2,
			OriginState:  domain.DivergenceAhead,
			IsDirty:      true,
			NeedsSync:    true,
			InCycle:      true,
		},
	}

	got := TreeBadges(node)
	want := []domain.TreeBadge{
		domain.TreeBadgeVirtual,
		domain.TreeBadgePR,
		domain.TreeBadgeAhead,
		domain.TreeBadgeOrigin,
		domain.TreeBadgeDirty,
		domain.TreeBadgeNeedsSync,
		domain.TreeBadgeCycle,
	}
	if len(got) != len(want) {
		t.Fatalf("badges = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("badge %d = %v, want %v", index, got[index], want[index])
		}
	}
}

// A paused rebase is the more precise signal, so it replaces "dirty" rather than
// stacking with it.
func TestTreeBadgesPreferRebasingOverDirty(t *testing.T) {
	node := domain.TreeNode{Status: domain.TreeNodeStatus{RebaseInProgress: true, IsDirty: true}}

	got := TreeBadges(node)
	if len(got) != 1 || got[0] != domain.TreeBadgeRebasing {
		t.Errorf("badges = %v, want the rebase alone", got)
	}
}

func TestTreeBadgesAreEmptyForACleanNode(t *testing.T) {
	if got := TreeBadges(domain.TreeNode{Branch: "main", IsMain: true}); len(got) != 0 {
		t.Errorf("badges = %v, want none", got)
	}
}

func TestTreeSpacerPrefixKeepsTheGutterRunning(t *testing.T) {
	cases := []struct {
		name       string
		nextPrefix string
		want       string
	}{
		{"first child of a root", domain.TreeConnectorBranch, domain.TreeGutterPipe},
		// The pipe runs down to and past the last sibling, or the line breaks just
		// above it.
		{"last sibling", domain.TreeConnectorLast, domain.TreeGutterPipe},
		{"nested", domain.TreeGutterPipe + domain.TreeConnectorBranch, domain.TreeGutterPipe + domain.TreeGutterPipe},
		{"a new root carries nothing", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TreeSpacerPrefix(tc.nextPrefix); got != tc.want {
				t.Errorf("spacer = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestForestWithRunningJobsStampsEveryNode(t *testing.T) {
	forest := domain.Forest{Roots: []domain.TreeNode{{
		Branch:   "main",
		Path:     "/wt/main",
		Children: []domain.TreeNode{{Branch: "feature", Path: "/wt/feature"}},
	}}}

	stamped := ForestWithRunningJobs(forest, map[string]int{"/wt/feature": 3})

	if got := stamped.Roots[0].Status.RunningJobs; got != 0 {
		t.Errorf("main carries %d running jobs, want none", got)
	}
	child := stamped.Roots[0].Children[0]
	if child.Status.RunningJobs != 3 {
		t.Errorf("feature carries %d running jobs, want 3", child.Status.RunningJobs)
	}
	if badges := TreeBadges(child); !hasBadge(badges, domain.TreeBadgeRunning) {
		t.Errorf("badges = %v, want the running one", badges)
	}
	if badges := TreeBadges(stamped.Roots[0]); hasBadge(badges, domain.TreeBadgeRunning) {
		t.Error("a worktree running nothing was badged as running")
	}
}

func hasBadge(badges []domain.TreeBadge, want domain.TreeBadge) bool {
	for _, badge := range badges {
		if badge == want {
			return true
		}
	}
	return false
}
