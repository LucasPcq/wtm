package identity_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/service/identity"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

func TestAllocateGivesTheSmallestFreeNumberAndSaysSo(t *testing.T) {
	repo, meta := fakes(t, map[string]domain.WorktreeMetadata{"feat/a": {Ordinal: 2, SourceBranch: "main"}}, "feat/a", "feat/b")

	claim, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, StateDir: t.TempDir(), Branch: "feat/b"})

	require.NoError(t, err)
	assert.Equal(t, identity.Claim{Ordinal: 1, Allocated: true}, claim)
	assert.Equal(t, 1, read(t, repo, meta).Records["feat/b"].(identity.Recorded).Meta.Ordinal)
}

func TestAllocateKeepsASettledNumberAndWritesNothing(t *testing.T) {
	repo, meta := fakes(t, map[string]domain.WorktreeMetadata{"feat/a": {Ordinal: 3}}, "feat/a")

	claim, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, StateDir: t.TempDir(), Branch: "feat/a"})

	require.NoError(t, err)
	assert.Equal(t, identity.Claim{Ordinal: 3}, claim)
	assert.NotContains(t, meta.Calls(), "Update feat/a")
}

func TestAllocateNeverWritesTheMainCheckout(t *testing.T) {
	repo, meta := fakes(t, nil)

	claim, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, StateDir: t.TempDir(), Branch: "main"})

	require.NoError(t, err)
	assert.Equal(t, identity.Claim{Ordinal: domain.MainWorktreeOrdinal}, claim)
	assert.NotContains(t, meta.Calls(), "Update main")
}

func TestAllocateMovesTheSecondOfTwoWorktreesClaimingANumber(t *testing.T) {
	repo, meta := fakes(t, map[string]domain.WorktreeMetadata{"feat/a": {Ordinal: 1}, "feat/b": {Ordinal: 1}}, "feat/a", "feat/b")

	claim, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, StateDir: t.TempDir(), Branch: "feat/b"})

	require.NoError(t, err)
	assert.Equal(t, identity.Claim{Ordinal: 2, Allocated: true}, claim)
}

func TestAllocateRefusesToNumberPastAnUnreadableNeighbour(t *testing.T) {
	repo, _ := fakes(t, nil, "feat/a", "feat/b")
	meta := storetest.NewFake(storetest.FakeParams{Fail: map[string]error{"Load": errors.New("broken")}})

	_, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, StateDir: t.TempDir(), Branch: "feat/b"})

	require.ErrorIs(t, err, domain.ErrOrdinalUnreadable)
	assert.NotContains(t, meta.Calls(), "Update feat/b")
}

func TestAllocateRefusesToNumberPastANeighbourWhoseRecordDoesNotRead(t *testing.T) {
	repo, _ := fakes(t, nil, "feat/a", "feat/b")
	meta := storetest.NewFake(storetest.FakeParams{Unreadable: []string{"feat/a"}})

	_, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, StateDir: t.TempDir(), Branch: "feat/b"})

	require.ErrorIs(t, err, domain.ErrOrdinalUnreadable)
	assert.NotContains(t, meta.Calls(), "Update feat/b")
}

func TestAllocateSaysItRanWithoutItsLock(t *testing.T) {
	repo, meta := fakes(t, nil, "feat/a")
	stateDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(stateDir, nil, 0o644))

	claim, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, StateDir: stateDir, Branch: "feat/a"})

	require.NoError(t, err)
	assert.Equal(t, identity.Claim{Ordinal: 1, Allocated: true, Unlocked: true}, claim)
}

func TestAllocateNeedsAStateDirAndABranch(t *testing.T) {
	repo, meta := fakes(t, nil)

	_, err := identity.Allocate(t.Context(), identity.AllocateParams{Git: repo, Meta: meta, Branch: "feat/a"})

	require.ErrorIs(t, err, domain.ErrOrdinalRefIncomplete)
}

func TestAllocateFromSeveralProcessesNeverGivesANumberTwice(t *testing.T) {
	branches := []string{"feat/a", "feat/b", "feat/c", "feat/d"}
	repo := repoWithWorktrees(t, branches...)
	stateDir := t.TempDir()
	const perBranch = 2

	var wg sync.WaitGroup
	for _, branch := range branches {
		for range perBranch {
			wg.Go(func() {
				cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
				cmd.Env = append(os.Environ(), envStateDir+"="+stateDir, envRepo+"="+repo, envBranch+"="+branch)
				out, err := cmd.CombinedOutput()
				assert.NoError(t, err, string(out))
			})
		}
	}
	wg.Wait()

	snapshot := read(t, git.Repo{Dir: repo, Remote: "origin"}, store.MetaDisk{StateDir: stateDir})
	given := map[int]string{}
	for _, branch := range branches {
		ordinal, err := snapshot.Ordinal(branch)
		require.NoError(t, err, branch)
		assert.NotContains(t, given, ordinal, "%s got %d, already %s's", branch, ordinal, given[ordinal])
		given[ordinal] = branch
	}
	assert.Len(t, given, len(branches))
}
