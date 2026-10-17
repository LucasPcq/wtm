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
		"checked out elsewhere":                   {PlacementParams{Branch: "feat/x", CheckedOutAt: "/wt"}, CheckedOutElsewhere{Path: "/wt"}},
		"checked out, if not exists":              {PlacementParams{Branch: "feat/x", CheckedOutAt: "/wt", IfNotExists: true}, Unchanged{Path: "/wt"}},
		"path taken by a folder":                  {PlacementParams{Branch: "feat/x", Path: path, Location: taken}, PathTaken{}},
		"path taken by the branch":                {PlacementParams{Branch: "feat/x", Path: path, Location: taken, Occupant: "feat/x"}, PathTaken{}},
		"path taken by the branch, if not exists": {PlacementParams{Branch: "feat/x", Path: path, Location: taken, Occupant: "feat/x", IfNotExists: true}, Unchanged{Path: path}},
		"path taken by a folder, if not exists":   {PlacementParams{Branch: "feat/x", Path: path, Location: taken, IfNotExists: true}, PathTaken{}},
		"path taken by another branch":            {PlacementParams{Branch: "feat/x", Path: path, Location: taken, Occupant: "feat-x"}, Clash{Branch: "feat-x", Name: "feat-x"}},
		"name clash with a live worktree":         {PlacementParams{Branch: "feat/x", Path: path, Live: []string{"main", "feat-x"}}, Clash{Branch: "feat-x", Name: "feat-x"}},
		"a local branch is checked out":           {PlacementParams{Branch: "feat/x", Path: path, Local: true, Remote: true}, Placed{Start: StartCheckOut}},
		"a branch on origin is resumed":           {PlacementParams{Branch: "feat/x", Path: path, Remote: true}, Placed{Start: StartResume, Upstream: "origin/feat/x"}},
		"a new branch is created":                 {PlacementParams{Branch: "feat/x", Path: path, Live: []string{"main"}}, Placed{Start: StartCreate}},
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
