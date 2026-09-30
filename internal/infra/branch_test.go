package infra

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestListLocalBranches(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "feature/login")
	gittest.CreateBranch(t, dir, "develop")

	branches, err := ListLocalBranches(ListBranchesParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(branches) < 3 {
		t.Fatalf("expected at least 3 branches (main/master, feature/login, develop), got %d: %v", len(branches), branches)
	}

	found := map[string]bool{}
	for _, b := range branches {
		found[b] = true
	}

	if !found["feature/login"] {
		t.Error("missing feature/login branch")
	}
	if !found["develop"] {
		t.Error("missing develop branch")
	}
}

// createRemoteRef fakes an origin remote-tracking branch by writing the ref
// directly, avoiding the need for a real remote in tests.
func createRemoteRef(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "update-ref", "refs/remotes/origin/"+name, "HEAD")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-ref %s: %s: %v", name, out, err)
	}
}

func TestListRemoteBranches(t *testing.T) {
	dir := gittest.InitRepo(t)
	createRemoteRef(t, dir, "feature/api")
	createRemoteRef(t, dir, "release")
	createRemoteRef(t, dir, "HEAD") // the symbolic pointer must be excluded

	branches, err := ListRemoteBranches(ListBranchesParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, b := range branches {
		found[b] = true
	}
	if !found["origin/feature/api"] || !found["origin/release"] {
		t.Errorf("missing expected remote branches, got %v", branches)
	}
	if found["origin/HEAD"] {
		t.Error("origin/HEAD must be excluded from remote branches")
	}
	// git shortens refs/remotes/origin/HEAD to the bare "origin"; it must not leak
	// in as a selectable branch.
	if found["origin"] {
		t.Error("the origin/HEAD pointer (shortened to \"origin\") must be excluded")
	}
}

func TestBranchOrRemoteExists(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "local-feat")
	createRemoteRef(t, dir, "remote-feat")

	if !BranchOrRemoteExists(BranchOrRemoteExistsParams{ProjectDir: dir, Ref: "local-feat"}) {
		t.Error("expected local branch to be found")
	}
	if !BranchOrRemoteExists(BranchOrRemoteExistsParams{ProjectDir: dir, Ref: "origin/remote-feat"}) {
		t.Error("expected origin remote-tracking branch to be found")
	}
	if BranchOrRemoteExists(BranchOrRemoteExistsParams{ProjectDir: dir, Ref: "origin/nope"}) {
		t.Error("nonexistent ref must not be found")
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := gittest.InitRepo(t)

	branch, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}

	if branch != "main" && branch != "master" {
		t.Errorf("expected default branch (main or master), got %s", branch)
	}
}

func TestCommitsAhead(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "feature")

	cmd := exec.Command("git", "checkout", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %s: %v", out, err)
	}

	commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "ahead")
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %s: %v", out, err)
	}

	branchCmd := exec.Command("git", "branch", "--format=%(refname:short)")
	branchCmd.Dir = dir
	branchOut, _ := branchCmd.Output()
	var baseBranch string
	for _, b := range strings.Split(strings.TrimSpace(string(branchOut)), "\n") {
		if b != "feature" {
			baseBranch = b
			break
		}
	}

	count, err := CommitsAhead(CommitsAheadParams{
		WorktreePath: dir,
		BaseBranch:   baseBranch,
		Branch:       "feature",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 commit ahead, got %d", count)
	}
}

func TestCommitsAhead_NoneAhead(t *testing.T) {
	dir := gittest.InitRepo(t)

	branch, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gittest.CreateBranch(t, dir, "no-ahead")

	count, err := CommitsAhead(CommitsAheadParams{
		WorktreePath: dir,
		BaseBranch:   branch,
		Branch:       "no-ahead",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 commits ahead, got %d", count)
	}
}

func TestAheadBehind(t *testing.T) {
	dir := gittest.InitRepo(t)

	branch, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pin origin/<branch> at the current commit, then add two local commits:
	// the local branch is now 2 ahead and 0 behind its origin counterpart.
	createRemoteRef(t, dir, branch)
	for i := 0; i < 2; i++ {
		commit := exec.Command("git", "commit", "--allow-empty", "-m", "ahead")
		commit.Dir = dir
		if out, err := commit.CombinedOutput(); err != nil {
			t.Fatalf("git commit: %s: %v", out, err)
		}
	}

	got, err := AheadBehind(AheadBehindParams{
		ProjectDir: dir,
		Local:      branch,
		Remote:     "origin/" + branch,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Ahead != 2 || got.Behind != 0 {
		t.Errorf("expected ahead 2 / behind 0, got %+v", got)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
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

func TestDeleteLocalBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "to-delete")

	err := DeleteLocalBranch(DeleteLocalBranchParams{
		ProjectDir: dir,
		Branch:     "to-delete",
		Force:      false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	branches, _ := ListLocalBranches(ListBranchesParams{ProjectDir: dir})
	for _, b := range branches {
		if b == "to-delete" {
			t.Error("branch should have been deleted")
		}
	}
}

// repoWithRemote returns a repo wired to a bare origin, with main pushed.
func repoWithRemote(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	remote := t.TempDir()
	runGit(t, remote, "init", "--bare", "-b", "main")
	runGit(t, dir, "remote", "add", "origin", remote)
	runGit(t, dir, "push", "-u", "origin", "main")
	return dir
}

// advanceOriginOf pushes one commit onto origin/<branch> from a detached
// worktree, leaving the local ref behind.
func advanceOriginOf(t *testing.T, dir, branch string) {
	t.Helper()
	tmp := t.TempDir()
	runGit(t, dir, "worktree", "add", "--detach", tmp, branch)
	runGit(t, tmp, "commit", "--allow-empty", "-m", "remote-only")
	runGit(t, tmp, "push", "origin", "HEAD:"+branch)
	runGit(t, dir, "worktree", "remove", "--force", tmp)
}

func TestFastForwardRef(t *testing.T) {
	dir := repoWithRemote(t)
	runGit(t, dir, "branch", "feat")
	runGit(t, dir, "push", "-u", "origin", "feat")
	advanceOriginOf(t, dir, "feat")

	if err := FastForwardRef(FastForwardRefParams{ProjectDir: dir, Branch: "feat"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := revParse(t, dir, "feat"), revParse(t, dir, "origin/feat"); got != want {
		t.Errorf("feat = %s, want origin/feat = %s", got, want)
	}
}

// The refspec carries no leading '+', so git itself refuses a non-fast-forward.
// This is the safety net the caller's ancestor checks rest on: it holds even if
// those checks were wrong.
func TestFastForwardRefRefusesNonFastForward(t *testing.T) {
	dir := repoWithRemote(t)
	runGit(t, dir, "branch", "feat")
	runGit(t, dir, "push", "-u", "origin", "feat")
	advanceOriginOf(t, dir, "feat")

	// Move the local ref sideways so it can no longer fast-forward.
	tmp := t.TempDir()
	runGit(t, dir, "worktree", "add", "--detach", tmp, "feat")
	runGit(t, tmp, "commit", "--allow-empty", "-m", "local-only")
	sideways := revParse(t, tmp, "HEAD")
	runGit(t, dir, "worktree", "remove", "--force", tmp)
	runGit(t, dir, "branch", "-f", "feat", sideways)

	if err := FastForwardRef(FastForwardRefParams{ProjectDir: dir, Branch: "feat"}); err == nil {
		t.Fatal("a non-fast-forward update must be refused")
	}
	if got := revParse(t, dir, "feat"); got != sideways {
		t.Errorf("feat moved to %s despite the refusal (was %s)", got, sideways)
	}
}

// A branch checked out in a worktree cannot be moved from under it; such parents
// are advanced with FastForwardBranch inside their own worktree instead.
func TestFastForwardRefRefusesCheckedOutBranch(t *testing.T) {
	dir := repoWithRemote(t)
	runGit(t, dir, "branch", "feat")
	runGit(t, dir, "push", "-u", "origin", "feat")
	advanceOriginOf(t, dir, "feat")

	wt := t.TempDir()
	runGit(t, dir, "worktree", "add", wt, "feat")
	before := revParse(t, dir, "feat")

	if err := FastForwardRef(FastForwardRefParams{ProjectDir: dir, Branch: "feat"}); err == nil {
		t.Fatal("git should refuse to fetch into a checked-out branch")
	}
	if got := revParse(t, dir, "feat"); got != before {
		t.Errorf("feat moved to %s despite the refusal (was %s)", got, before)
	}
}
