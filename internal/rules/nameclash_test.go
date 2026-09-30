package rules

import "testing"

func TestWorktreeNameClash(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		live   []string
		other  string
		shared string
	}{
		{name: "a dot and a slash share the slug", branch: "feat.x", live: []string{"main", "feat/x"}, other: "feat/x", shared: "feat-x"},
		{name: "case is folded", branch: "Feat/X", live: []string{"feat/x"}, other: "feat/x", shared: "feat-x"},
		{name: "an underscore survives the slug, not the host label", branch: "feat/a_b", live: []string{"feat/a-b"}, other: "feat/a-b", shared: "feat-a-b"},
		{name: "the branch itself is no clash", branch: "feat/x", live: []string{"feat/x"}},
		{name: "distinct names", branch: "feat/y", live: []string{"main", "feat/x"}},
		{name: "a detached worktree has no name", branch: "feat/x", live: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clash, found := WorktreeNameClash(WorktreeNameClashParams{Branch: tt.branch, Live: tt.live})
			if found != (tt.other != "") {
				t.Fatalf("found = %v, clash = %+v", found, clash)
			}
			if clash.Branch != tt.other || clash.Name != tt.shared {
				t.Errorf("clash = %+v, want %s (%s)", clash, tt.other, tt.shared)
			}
		})
	}
}
