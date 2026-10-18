package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhereABranchAskedForANewWorktreeLands(t *testing.T) {
	path := "/trees/feat-x"
	taken := Location{Path: path, Exists: true}
	cases := map[string]struct {
		params PlacementParams
		want   Placement
	}{
		"checked_out_elsewhere":              {PlacementParams{Branch: "feat/x", CheckedOutAt: "/wt"}, CheckedOutElsewhere{Path: "/wt"}},
		"checked_out_if_not_exists":          {PlacementParams{Branch: "feat/x", CheckedOutAt: "/wt", IfNotExists: true}, Unchanged{Path: "/wt"}},
		"path_taken_by_folder":               {PlacementParams{Branch: "feat/x", Path: path, Location: taken}, PathTaken{}},
		"path_taken_by_branch":               {PlacementParams{Branch: "feat/x", Path: path, Location: taken, Occupant: "feat/x"}, PathTaken{}},
		"path_taken_by_branch_if_not_exists": {PlacementParams{Branch: "feat/x", Path: path, Location: taken, Occupant: "feat/x", IfNotExists: true}, Unchanged{Path: path}},
		"path_taken_by_folder_if_not_exists": {PlacementParams{Branch: "feat/x", Path: path, Location: taken, IfNotExists: true}, PathTaken{}},
		"path_taken_by_other_branch":         {PlacementParams{Branch: "feat/x", Path: path, Location: taken, Occupant: "feat-x"}, Clash{Branch: "feat-x", Name: "feat-x"}},
		"name_clash_live_worktree":           {PlacementParams{Branch: "feat/x", Path: path, Live: []string{"main", "feat-x"}}, Clash{Branch: "feat-x", Name: "feat-x"}},
		"local_branch_checked_out":           {PlacementParams{Branch: "feat/x", Path: path, Local: true, Remote: true}, Placed{Start: StartCheckOut}},
		"origin_branch_resumed":              {PlacementParams{Branch: "feat/x", Path: path, Remote: true}, Placed{Start: StartResume, Upstream: "origin/feat/x"}},
		"new_branch_created":                 {PlacementParams{Branch: "feat/x", Path: path, Live: []string{"main"}}, Placed{Start: StartCreate}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, Place(c.params))
		})
	}
}

func TestAPlacedBranchIsCheckedOutAsItStarts(t *testing.T) {
	from := LocalBranch{Name: "main"}
	assert.Equal(t, ExistingBranch{Name: "feat/x"}, Placed{Start: StartCheckOut}.Checkout("feat/x", from))
	assert.Equal(t, TrackingBranch{Name: "feat/x"}, Placed{Start: StartResume, Upstream: "origin/feat/x"}.Checkout("feat/x", from))
	assert.Equal(t, NewBranch{Name: "feat/x", From: from}, Placed{Start: StartCreate}.Checkout("feat/x", from))
}
