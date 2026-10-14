package identity_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/service/identity"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

const (
	envStateDir = "WTM_TEST_IDENTITY_STATE_DIR"
	envRepo     = "WTM_TEST_IDENTITY_REPO"
	envBranch   = "WTM_TEST_IDENTITY_BRANCH"
)

func read(t *testing.T, repo git.Reader, meta store.Meta) identity.Snapshot {
	t.Helper()
	snapshot, err := identity.Read(t.Context(), identity.ReadParams{Git: repo, Meta: meta})
	require.NoError(t, err)
	return snapshot
}

// TestHelperProcess is the body of the processes the cross-process test
// starts: it allocates its branch a number, and is skipped otherwise.
func TestHelperProcess(t *testing.T) {
	stateDir := os.Getenv(envStateDir)
	if stateDir == "" {
		t.Skip("run by TestAllocateFromSeveralProcessesNeverGivesANumberTwice")
	}
	_, err := identity.Allocate(t.Context(), identity.AllocateParams{
		Git:      git.Repo{Dir: os.Getenv(envRepo), Remote: "origin"},
		Meta:     store.MetaDisk{StateDir: stateDir},
		StateDir: stateDir,
		Branch:   os.Getenv(envBranch),
	})
	require.NoError(t, err)
}

func repoWithWorktrees(t *testing.T, branches ...string) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	for _, branch := range branches {
		gittest.Git(t, dir, "worktree", "add", "-q", "-b", branch, filepath.Join(t.TempDir(), branch))
	}
	return dir
}
