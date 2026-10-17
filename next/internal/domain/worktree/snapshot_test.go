package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var snapshot = Snapshot{
	Worktrees: []Worktree{
		{Path: "/repo", Main: true, State: OnBranch{Branch: "main"}},
		{Path: "/trees/feat-x", State: OnBranch{Branch: "feat/x"}},
		{Path: "/trees/detached", State: Detached{}},
	},
	Branches: Branches{
		Local:  []Branch{{Name: "main", Tip: "a"}, {Name: "feat/x", Tip: "b"}, {Name: "ahead", Tip: "c"}},
		Remote: []Branch{{Name: "main", Tip: "z"}, {Name: "ahead", Tip: "y"}, {Name: "remote-only", Tip: "x"}, {Name: "same", Tip: "s"}},
	},
	Divergence: map[string]AheadBehind{"main": {Behind: 2}, "ahead": {Ahead: 1}},
}

func TestASnapshotKnowsItsBranches(t *testing.T) {
	cases := map[string]struct {
		name                     string
		local, remote, candidate bool
	}{
		"local and on origin":  {name: "main", local: true, remote: true, candidate: true},
		"local only":           {name: "feat/x", local: true, candidate: true},
		"on origin only":       {name: "remote-only", remote: true},
		"origin/ of a branch":  {name: "origin/remote-only", candidate: true},
		"origin/ of no branch": {name: "origin/nope"},
		"unknown":              {name: "nope"},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			assert.Equal(t, c.local, snapshot.Local(c.name))
			assert.Equal(t, c.remote, snapshot.Remote(c.name))
			assert.Equal(t, c.candidate, snapshot.Candidate(c.name))
		})
	}
}

func TestASnapshotStartsFromOriginOnlyWithoutALocalBranch(t *testing.T) {
	assert.Equal(t, RemoteBranch{Name: "remote-only"}, snapshot.Ref("origin/remote-only"))
	assert.Equal(t, LocalBranch{Name: "main"}, snapshot.Ref("main"))
	assert.Equal(t, LocalBranch{Name: "origin/x"}, Snapshot{Branches: Branches{Local: []Branch{{Name: "origin/x"}}}}.Ref("origin/x"))
}

func TestASnapshotFindsWhereABranchIsCheckedOutAndWhoOccupiesAPath(t *testing.T) {
	assert.Equal(t, "/trees/feat-x", snapshot.CheckedOutAt("feat/x"))
	assert.Empty(t, snapshot.CheckedOutAt("ahead"))
	assert.Equal(t, "feat/x", snapshot.Occupant(Location{Path: "/trees/feat-x", Exists: true}))
	assert.Empty(t, snapshot.Occupant(Location{Path: "/trees/detached", Exists: true}))
	assert.Empty(t, snapshot.Occupant(Location{Path: "/trees/free"}))
	assert.Equal(t, []string{"main", "feat/x", ""}, snapshot.Live())
}

func TestASnapshotSaysWhereABranchStandsAgainstOrigin(t *testing.T) {
	cases := map[string]struct {
		name   string
		want   Divergence
		counts AheadBehind
	}{
		"behind":        {name: "main", want: Behind, counts: AheadBehind{Behind: 2}},
		"ahead":         {name: "ahead", want: Ahead, counts: AheadBehind{Ahead: 1}},
		"same tip":      {name: "same", want: UpToDate},
		"not on origin": {name: "feat/x", want: NoOrigin},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			assert.Equal(t, c.want, snapshot.Standing(c.name))
			assert.Equal(t, c.counts, snapshot.AheadBehind(c.name))
		})
	}
}

func TestOnlyTheLocalBranchesWhoseOriginTipDiffersAreCounted(t *testing.T) {
	assert.Equal(t, []string{"main", "ahead"}, snapshot.Branches.Differing())
	assert.Nil(t, Branches{Local: []Branch{{Name: "main", Tip: "a"}}, Remote: []Branch{{Name: "main", Tip: "a"}}}.Differing())
}
