package branch

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// repoWithRemote returns a work repo (on main) wired to a fresh bare origin, with
// main pushed so origin-tracking refs exist.
func repoWithRemote(t *testing.T) string {
	t.Helper()
	work := gittest.InitRepo(t)
	origin := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", origin)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}
	git(t, work, "remote", "add", "origin", origin)
	git(t, work, "push", "-u", "origin", "main")
	return work
}

func TestFastForwardToOriginAdvancesBehindBranch(t *testing.T) {
	work := repoWithRemote(t)

	// "feat" at base, pushed; origin then advances feat by one commit while the
	// local feat stays put and unchecked out (HEAD on main).
	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:feat")

	if err := FastForwardToOrigin(t.Context(), BranchParams{ProjectDir: work, Branch: "feat"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := revParse(t, work, "feat"), revParse(t, work, "origin/feat"); got != want {
		t.Errorf("feat = %s, want origin/feat = %s", got, want)
	}
}

func TestFastForwardToOriginRefusesDiverged(t *testing.T) {
	work := repoWithRemote(t)

	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")

	// local feat gains a commit...
	git(t, work, "checkout", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "local-commit")
	local := revParse(t, work, "feat")
	git(t, work, "checkout", "main")

	// ...and origin's feat gains a different commit → diverged.
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:feat")

	err := FastForwardToOrigin(t.Context(), BranchParams{ProjectDir: work, Branch: "feat"})
	if err == nil {
		t.Fatal("expected a diverged error, got nil")
	}
	if got := revParse(t, work, "feat"); got != local {
		t.Errorf("feat was modified (%s), want unchanged %s", got, local)
	}
}

func TestCheckReportsBehindBranch(t *testing.T) {
	work := repoWithRemote(t)
	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:feat")

	check, err := Check(t.Context(), BranchParams{ProjectDir: work, Branch: "feat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !check.HasUpstream {
		t.Fatal("HasUpstream = false, want true")
	}
	if check.Behind != 1 || check.Ahead != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 0/1", check.Ahead, check.Behind)
	}
	if check.State != domain.DivergenceBehind {
		t.Fatalf("state = %v, want DivergenceBehind", check.State)
	}
}

func TestCheckReportsNoUpstream(t *testing.T) {
	work := repoWithRemote(t)
	git(t, work, "branch", "local-only")

	check, err := Check(t.Context(), BranchParams{ProjectDir: work, Branch: "local-only"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.HasUpstream {
		t.Fatal("HasUpstream = true, want false")
	}
	if check.State != domain.DivergenceUnknown {
		t.Fatalf("state = %v, want DivergenceUnknown", check.State)
	}
}

func TestFastForwardAdvancesBranchWithNoWorktree(t *testing.T) {
	work := repoWithRemote(t)
	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:feat")

	result := FastForward(t.Context(), FastForwardParams{ProjectDir: work, Branch: "feat"})
	if result.Status != domain.FFAdvanced {
		t.Fatalf("status = %v (%s), want FFAdvanced", result.Status, result.Detail)
	}
	if got, want := revParse(t, work, "feat"), revParse(t, work, "origin/feat"); got != want {
		t.Fatalf("feat = %s, want %s", got, want)
	}
	if result.OldTip == result.NewTip {
		t.Fatalf("tips unchanged: %s", result.NewTip)
	}
	if result.Label != domain.FastForwardLabelAdvanced {
		t.Fatalf("label = %q, want %q", result.Label, domain.FastForwardLabelAdvanced)
	}
}

func TestFastForwardIsANoOpWhenUpToDate(t *testing.T) {
	work := repoWithRemote(t)
	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")

	result := FastForward(t.Context(), FastForwardParams{ProjectDir: work, Branch: "feat"})
	if result.Status != domain.FFUpToDate {
		t.Fatalf("status = %v (%s), want FFUpToDate", result.Status, result.Detail)
	}
	if result.OldTip != result.NewTip {
		t.Fatalf("tips moved: %s → %s", result.OldTip, result.NewTip)
	}
}

func TestFastForwardRefusesDivergedEvenWithForce(t *testing.T) {
	work := repoWithRemote(t)
	git(t, work, "branch", "feat")
	git(t, work, "push", "origin", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:feat")
	git(t, work, "checkout", "feat")
	git(t, work, "commit", "--allow-empty", "-m", "local-commit")
	localTip := revParse(t, work, "feat")

	result := FastForward(t.Context(), FastForwardParams{ProjectDir: work, Branch: "feat", Force: true})
	if result.Status != domain.FFDiverged {
		t.Fatalf("status = %v (%s), want FFDiverged", result.Status, result.Detail)
	}
	if got := revParse(t, work, "feat"); got != localTip {
		t.Fatalf("feat moved to %s, want it left at %s", got, localTip)
	}
}

func TestFastForwardRefusesNoUpstream(t *testing.T) {
	work := repoWithRemote(t)
	git(t, work, "branch", "local-only")

	result := FastForward(t.Context(), FastForwardParams{ProjectDir: work, Branch: "local-only"})
	if result.Status != domain.FFNoUpstream {
		t.Fatalf("status = %v (%s), want FFNoUpstream", result.Status, result.Detail)
	}
}
