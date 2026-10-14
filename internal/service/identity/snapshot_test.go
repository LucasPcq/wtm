package identity_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/service/identity"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

func TestASnapshotHoldsTheRecordOfEachLiveWorktree(t *testing.T) {
	repo, meta := fakes(t, map[string]domain.WorktreeMetadata{"feat/a": {Ordinal: 1}, "gone": {Ordinal: 2}}, "feat/a", "feat/b")

	snapshot := read(t, repo, meta)

	assert.Equal(t, map[string]identity.Record{
		"main":   identity.Unrecorded{},
		"feat/a": identity.Recorded{Meta: domain.WorktreeMetadata{Ordinal: 1}},
		"feat/b": identity.Unrecorded{},
	}, snapshot.Records)
}

func TestARecordThatCannotBeReadIsKeptAsUnreadable(t *testing.T) {
	broken := errors.New("broken")
	repo := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main"})
	meta := storetest.NewFake(storetest.FakeParams{Fail: map[string]error{"Load": broken}})

	snapshot := read(t, repo, meta)

	assert.Equal(t, identity.Unreadable{Err: broken}, snapshot.Records["main"])
}

func TestReadingTwiceWritesNothing(t *testing.T) {
	repo, meta := fakes(t, map[string]domain.WorktreeMetadata{"feat/a": {Ordinal: 1}}, "feat/a")
	before := len(repo.Calls())

	for range 2 {
		snapshot := read(t, repo, meta)
		_, _ = snapshot.Ordinal("feat/a")
		_ = snapshot.Isolation("feat/a")
		_, _ = snapshot.NameClash("feat-a")
		_ = snapshot.Managed("feat/a")
	}

	for _, call := range append(repo.Calls()[before:], meta.Calls()...) {
		assert.True(t, strings.HasPrefix(call, "Worktrees") || strings.HasPrefix(call, "Load"), call)
	}
}

func TestTheMainCheckoutIsZeroAndAnUnnumberedWorktreeUnallocated(t *testing.T) {
	snapshot := snapshotOf(t, nil, "feat/a")

	main, err := snapshot.Ordinal("main")
	require.NoError(t, err)
	assert.Equal(t, domain.MainWorktreeOrdinal, main)
	_, err = snapshot.Ordinal("feat/a")
	assert.ErrorIs(t, err, domain.ErrOrdinalUnallocated)
}

func TestOfTwoWorktreesClaimingANumberTheFirstBranchKeepsIt(t *testing.T) {
	snapshot := snapshotOf(t, map[string]domain.WorktreeMetadata{"feat/a": {Ordinal: 1}, "feat/b": {Ordinal: 1}}, "feat/a", "feat/b")

	kept, err := snapshot.Ordinal("feat/a")
	require.NoError(t, err)
	assert.Equal(t, 1, kept)
	_, err = snapshot.Ordinal("feat/b")
	assert.ErrorIs(t, err, domain.ErrOrdinalUnallocated)
}

func TestAnUnreadableNeighbourFailsTheNumberButNotTheRest(t *testing.T) {
	snapshot := identity.Snapshot{
		Worktrees: []git.Worktree{
			{Path: "/repo", Main: true, State: git.OnBranch{Branch: "main"}},
			{Path: "/wt/a", State: git.OnBranch{Branch: "feat/a"}},
			{Path: "/wt/b", State: git.OnBranch{Branch: "feat/b"}},
		},
		Records: map[string]identity.Record{
			"main":   identity.Unrecorded{},
			"feat/a": identity.Recorded{Meta: domain.WorktreeMetadata{Ordinal: 1, Isolation: domain.IsolationVerbatim}},
			"feat/b": identity.Unreadable{Err: errors.New("broken")},
		},
	}

	_, err := snapshot.Ordinal("feat/a")

	assert.ErrorIs(t, err, domain.ErrOrdinalUnreadable)
	assert.Equal(t, domain.IsolationVerbatim, snapshot.Isolation("feat/a"))
	assert.Equal(t, domain.IsolationIsolated, snapshot.Isolation("feat/b"))
}

func TestAWorktreeWithoutARecordIsIsolatedAndUnmanaged(t *testing.T) {
	snapshot := snapshotOf(t, map[string]domain.WorktreeMetadata{"feat/a": {CreatedAt: "2026-10-12T10:00:00Z"}}, "feat/a", "feat/b")

	assert.Equal(t, domain.IsolationIsolated, snapshot.Isolation("feat/b"))
	assert.True(t, snapshot.Managed("feat/a"))
	assert.False(t, snapshot.Managed("feat/b"))
}

func TestTheMainCheckoutIsTheFirstWorktreesBranch(t *testing.T) {
	snapshot := snapshotOf(t, nil, "feat/a")

	assert.True(t, snapshot.IsMain("main"))
	assert.False(t, snapshot.IsMain("feat/a"))
	assert.False(t, snapshot.IsMain("nope"))
}

func TestABranchWhoseNamesALiveWorktreeCarriesClashes(t *testing.T) {
	snapshot := snapshotOf(t, nil, "feat/a")

	clash, found := snapshot.NameClash("feat-a")

	require.True(t, found)
	assert.Equal(t, domain.WorktreeNameClash{Branch: "feat/a", Name: "feat-a"}, clash)
	_, found = snapshot.NameClash("feat/b")
	assert.False(t, found)
}

func TestWhereABranchIsOutAndWhatAFolderHolds(t *testing.T) {
	snapshot := snapshotOf(t, nil, "feat/a")

	assert.Equal(t, "/wt/feat/a", snapshot.CheckedOutAt("feat/a"))
	assert.Empty(t, snapshot.CheckedOutAt("feat/b"))
	assert.Equal(t, "feat/a", snapshot.Occupant(git.Location{Path: "/wt/feat/a", Exists: true}))
	assert.Empty(t, snapshot.Occupant(git.Location{Path: "/wt/feat-a"}))
}

func TestAFolderReachedThroughASymlinkHoldsTheBranchGitRecorded(t *testing.T) {
	dir := gittest.InitRepo(t)
	path := filepath.Join(gittest.Symlinked(t), "feat-a")
	gittest.Git(t, dir, "worktree", "add", "-q", "-b", "feat/a", path)
	repo := git.Repo{Dir: dir, Remote: "origin"}
	at, err := repo.Locate(t.Context(), path)
	require.NoError(t, err)

	snapshot := read(t, repo, storetest.NewFake(storetest.FakeParams{}))

	assert.Equal(t, "feat/a", snapshot.Occupant(at))
	assert.Equal(t, at.Path, snapshot.CheckedOutAt("feat/a"))
}
