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
		"local_and_origin":    {name: "main", local: true, remote: true, candidate: true},
		"local_only":          {name: "feat/x", local: true, candidate: true},
		"origin_only":         {name: "remote-only", remote: true},
		"origin_of_branch":    {name: "origin/remote-only", candidate: true},
		"origin_of_no_branch": {name: "origin/nope"},
		"unknown":             {name: "nope"},
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
		"same_tip":      {name: "same", want: UpToDate},
		"not_on_origin": {name: "feat/x", want: NoOrigin},
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

func TestASourceBadgeSaysHowFarItIsFromOrigin(t *testing.T) {
	diverged := Snapshot{Branches: Branches{Remote: []Branch{{Name: "both"}}}, Divergence: map[string]AheadBehind{"both": {Ahead: 1, Behind: 3}}}
	assert.Equal(t, "2 behind", snapshot.Badge("main"))
	assert.Equal(t, "1 ahead", snapshot.Badge("ahead"))
	assert.Equal(t, "1 ahead, 3 behind", diverged.Badge("both"))
	assert.Empty(t, snapshot.Badge("same"))
	assert.Empty(t, snapshot.Badge("feat/x"))
}

func TestTheStandingOfAPlacedBranchIsItsCountsWhenCheckedOutAndItsUpstreamWhenResumed(t *testing.T) {
	cases := map[string]struct {
		branch string
		placed Placed
		want   Standing
	}{
		"checked_out": {"main", Placed{Start: StartCheckOut}, Standing{ExistingBranch: true, OriginState: Behind, OriginBehind: 2}},
		"resumed":     {"remote-only", Placed{Start: StartResume, Upstream: "origin/remote-only"}, Standing{Upstream: "origin/remote-only"}},
		"created":     {"main", Placed{Start: StartCreate}, Standing{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, snapshot.StandingOf(c.branch, c.placed))
		})
	}
}

func TestANewWorktreeStartsFromTheBranchItChecksOutOrTheLocalSource(t *testing.T) {
	snapshot := Snapshot{Branches: Branches{Local: []Branch{{Name: "main"}}}}

	assert.Equal(t, "feat/old", snapshot.StartsFrom("feat/old", StartCheckOut, "main"))
	assert.Equal(t, "main", snapshot.StartsFrom("feat/a", StartCreate, "main"))
	assert.Empty(t, snapshot.StartsFrom("feat/a", StartCreate, "origin/main"))
	assert.Empty(t, snapshot.StartsFrom("feat/r", StartResume, "main"))
}

func TestAnOriginIsSaidOnlyOfABranchOriginHas(t *testing.T) {
	assert.Equal(t, &Origin{Behind: 2, State: Behind}, snapshot.OriginOf("main"))
	assert.Equal(t, &Origin{State: UpToDate}, snapshot.OriginOf("same"))
	assert.Nil(t, snapshot.OriginOf("feat/x"))
	assert.Nil(t, snapshot.OriginOf(""))
}
