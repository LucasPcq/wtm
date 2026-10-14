package worktree

import (
	"os/exec"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// LUC-305: checkout names origin/<branch> as the start point, create its source;
// either way a branch only on origin starts from origin and tracks it.
func TestCreateTracksABranchOnlyOnOrigin(t *testing.T) {
	for _, from := range []string{"main", "origin/feat/remote"} {
		repo := newOccupiedRepo(t)
		gittest.AddOrigin(t, repo.dir)
		gittest.Git(t, repo.dir, "checkout", "-b", "feat/remote")
		gittest.Git(t, repo.dir, "commit", "--allow-empty", "-m", "pushed work")
		gittest.PushBranch(t, repo.dir, "feat/remote")
		gittest.Git(t, repo.dir, "checkout", "main")
		gittest.Git(t, repo.dir, "branch", "-D", "feat/remote")
		tip := gittest.Output(t, repo.dir, "rev-parse", "origin/feat/remote")

		result, err := Create(t.Context(), domain.CreateParams{
			ProjectDir: repo.dir, StateDir: repo.dir + "/.git/wtm", Branch: "feat/remote", FromBranch: from,
			SourceBranch: "main", Config: repo.config, SkipHooks: true,
		})
		if err != nil {
			t.Fatalf("from %s: %v", from, err)
		}
		if head := gittest.Output(t, result.Path, "rev-parse", "HEAD"); head != tip {
			t.Errorf("from %s: HEAD = %s, want origin's tip %s", from, head, tip)
		}
		if upstream := gittest.Output(t, result.Path, "rev-parse", "--abbrev-ref", "feat/remote@{upstream}"); upstream != "origin/feat/remote" {
			t.Errorf("from %s: upstream = %q, want origin/feat/remote", from, upstream)
		}
		if result.Upstream != "origin/feat/remote" || result.Metadata.SourceBranch != "main" {
			t.Errorf("from %s: result = %+v, want the upstream reported and main recorded as parent", from, result)
		}
	}
}

func TestCreateSetsNoUpstreamOnANewBranch(t *testing.T) {
	repo := newOccupiedRepo(t)
	gittest.AddOrigin(t, repo.dir)

	result, err := repo.create(t, "feat/new", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Upstream != "" {
		t.Errorf("upstream = %q, want none: a new branch must not pull from main", result.Upstream)
	}
	if out, err := exec.Command("git", "-C", repo.dir, "rev-parse", "--abbrev-ref", "feat/new@{upstream}").CombinedOutput(); err == nil {
		t.Errorf("upstream = %q, want no tracking set", out)
	}
}
