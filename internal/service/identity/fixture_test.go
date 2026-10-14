package identity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/service/identity"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

// fakes is a repository on main with a worktree for each branch, and their
// records.
func fakes(t *testing.T, records map[string]domain.WorktreeMetadata, branches ...string) (*gittest.Fake, *storetest.Fake) {
	t.Helper()
	repo := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main"})
	for _, branch := range branches {
		require.NoError(t, repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: "/wt/" + branch, Checkout: git.NewBranch{Name: branch, From: git.LocalBranch{Name: "main"}}}))
	}
	return repo, storetest.NewFake(storetest.FakeParams{Meta: records})
}

func snapshotOf(t *testing.T, records map[string]domain.WorktreeMetadata, branches ...string) identity.Snapshot {
	t.Helper()
	repo, meta := fakes(t, records, branches...)
	return read(t, repo, meta)
}
