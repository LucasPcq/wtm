package wt

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestCreateDisplayPathPrefersTheConfiguredLocation renders a fresh worktree as
// base_path/<name>, and keeps the real path for one living elsewhere.
func TestCreateDisplayPathPrefersTheConfiguredLocation(t *testing.T) {
	config := domain.Config{}
	config.Project.Worktrees.BasePath = "../.trees"

	inBase := createDisplayPath(displayPathParams{
		Config:     config,
		ProjectDir: "/repo",
		Path:       "/.trees/feat-x",
	})
	if inBase != "../.trees/feat-x" {
		t.Errorf("display path = %q, want the base_path form", inBase)
	}

	elsewhere := createDisplayPath(displayPathParams{
		Config:     config,
		ProjectDir: "/repo",
		Path:       "/somewhere/else/feat-x",
	})
	if elsewhere != "/somewhere/else/feat-x" {
		t.Errorf("display path = %q, want the real path for a foreign worktree", elsewhere)
	}
}

// behindBranch creates branch on main, pushes it, then advances origin so the
// local branch is behind by one commit.
func behindBranch(t *testing.T, work, name string) {
	t.Helper()
	gitRun(t, work, "branch", name)
	gitRun(t, work, "push", "origin", name)
	gitRun(t, work, "commit", "--allow-empty", "-m", "server-commit")
	gitRun(t, work, "push", "origin", "main:"+name)
	gitRun(t, work, "reset", "--hard", "HEAD~1")
	gitRun(t, work, "fetch", "origin")
}

// divergedBranch creates branch with one local commit origin does not have, while
// origin has one the local branch does not — one ahead, one behind.
func divergedBranch(t *testing.T, work, name string) {
	t.Helper()
	behindBranch(t, work, name)
	gitRun(t, work, "commit", "--allow-empty", "-m", "local-commit")
	gitRun(t, work, "branch", "-f", name, "HEAD")
	gitRun(t, work, "reset", "--hard", "HEAD~1")
}

// TestBehindAndDivergedFixtures guards the fixtures themselves: a wrong fixture
// would make the divergence assertions above pass for the wrong reason.
func TestBehindAndDivergedFixtures(t *testing.T) {
	work := repoWithRemote(t)
	behindBranch(t, work, "feat/b")
	divergedBranch(t, work, "feat/d")

	if got := revParse(t, work, "feat/b"); got == revParse(t, work, "origin/feat/b") {
		t.Error("the behind fixture should differ from its origin counterpart")
	}
	if revParse(t, work, "feat/d") == revParse(t, work, "origin/feat/d") {
		t.Error("the diverged fixture should differ from its origin counterpart")
	}
}
