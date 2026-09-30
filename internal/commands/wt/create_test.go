package wt

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestWtCreateFromRemoteBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	// Fake a remote-tracking branch that has no local counterpart.
	cmd := exec.Command("git", "update-ref", "refs/remotes/origin/upstream", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-ref: %s: %v", out, err)
	}

	branch := "feat/from-remote"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "origin/upstream", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create --from origin/upstream: %v", err)
	}

	if _, err := os.Stat(resolveWorktreePath(t, dir, branch)); err != nil {
		t.Fatalf("worktree not created from remote branch: %v", err)
	}
}

func TestWtCreateFromUnknownBranchFails(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/x", "--from", "origin/ghost", "--output", domain.OutputJSON, "--"+domain.FlagYes); err == nil {
		t.Fatal("expected an error for a nonexistent source branch")
	}
}

// LUC-172: a local branch that already exists is checked out as-is, keeping its
// commits. --from is not a start-point there — it is the parent recorded for sync.
func TestWtCreateReusesExistingLocalBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/existing"
	gittest.CreateBranch(t, dir, branch)
	gitCommitOn(t, dir, branch, "local-only work")
	want := revParse(t, dir, branch)

	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt create on an existing branch: %v", err)
	}

	var got domain.CreateResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode create JSON: %v (payload %q)", err, stdout)
	}
	if !got.ExistingBranch {
		t.Error("existing_branch should report the reuse")
	}
	if got.Metadata.SourceBranch != "main" {
		t.Errorf("source_branch = %q, want the --from value recorded as the sync parent", got.Metadata.SourceBranch)
	}

	wtPath := resolveWorktreePath(t, dir, branch)
	if head := revParse(t, wtPath, "HEAD"); head != want {
		t.Errorf("worktree HEAD = %s, want the branch tip %s — local commits were lost", head, want)
	}
}

// The branch axis is separate from the directory one: a branch another worktree
// already holds is refused with the typed error, not a raw git failure.
func TestWtCreateBranchCheckedOutElsewhereFails(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/taken"
	gittest.CreateBranch(t, dir, branch)
	gitRun(t, dir, "worktree", "add", filepath.Join(t.TempDir(), "taken"), branch)

	_, _, err := runWtCmd(t, domain.CmdCreate, branch, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected an error for a branch already checked out elsewhere")
	}
	if !errors.Is(err, domain.ErrWorktreeExists) {
		t.Errorf("error = %v, want it to wrap ErrWorktreeExists", err)
	}
	if code := rules.ExitCode(err); code != domain.ExitCodeWorktreeExists {
		t.Errorf("exit code = %d, want %d", code, domain.ExitCodeWorktreeExists)
	}
	if !strings.Contains(err.Error(), "wtm go "+branch) {
		t.Errorf("error %q should hint at `wtm go %s`", err, branch)
	}
}

// --if-not-exists is about the worktree, not the branch: a branch taken elsewhere
// resolves to that worktree instead of failing, so agents can retry safely.
func TestWtCreateIfNotExistsWithBranchCheckedOutElsewhere(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/taken"
	gittest.CreateBranch(t, dir, branch)
	existing := filepath.Join(t.TempDir(), "taken")
	gitRun(t, dir, "worktree", "add", existing, branch)

	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch,
		"--"+domain.FlagIfNotExists, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("--if-not-exists should succeed idempotently: %v", err)
	}

	var got domain.CreateResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode create JSON: %v (payload %q)", err, stdout)
	}
	if !got.AlreadyExists {
		t.Error("already_exists should be true")
	}
	// git resolves symlinks in worktree paths (e.g. macOS /var -> /private/var),
	// so compare the resolved form rather than the raw t.TempDir() value.
	wantPath, err := filepath.EvalSymlinks(existing)
	if err != nil {
		t.Fatalf("resolve want path: %v", err)
	}
	if got.Path != wantPath {
		t.Errorf("path = %q, want the worktree already holding the branch %q", got.Path, wantPath)
	}
}

// A branch that exists but is free is not an "already exists" case: --if-not-exists
// must still create the worktree.
func TestWtCreateIfNotExistsWithFreeExistingBranchCreates(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/free"
	gittest.CreateBranch(t, dir, branch)

	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main",
		"--"+domain.FlagIfNotExists, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt create --if-not-exists on a free existing branch: %v", err)
	}

	var got domain.CreateResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode create JSON: %v (payload %q)", err, stdout)
	}
	if got.AlreadyExists {
		t.Error("a free existing branch is not an already-existing worktree")
	}
	if !got.ExistingBranch {
		t.Error("existing_branch should report the reuse")
	}
	resolveWorktreePath(t, dir, branch)
}

// An existing branch was created outside wtm, so nothing says what it was branched
// off. Rather than record base_branch as a guess that sync and tree would treat as
// fact, the non-interactive path refuses and names the flag.
func TestWtCreateExistingBranchRequiresFrom(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/orphan"
	gittest.CreateBranch(t, dir, branch)

	_, _, err := runWtCmd(t, domain.CmdCreate, branch, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("an existing branch must not silently inherit base_branch as its parent")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagFrom) {
		t.Errorf("error %q should name the --%s flag", err, domain.FlagFrom)
	}

	// A branch git creates keeps its base_branch default — its start-point is a fact.
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/fresh", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Errorf("a new branch still defaults to the base branch: %v", err)
	}
}

// LUC-172: --yes is the confirmation axis, never a mutation trigger of its own —
// a reused branch behind origin stays exactly where it is; only --ff or the
// (unavailable, non-interactive) wizard prompt may move it.
func TestWtCreateYesDoesNotFastForwardBehindBranch(t *testing.T) {
	work := repoWithRemote(t)
	stateDir := filepath.Join(work, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, work)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/behind"
	gitRun(t, work, "branch", branch)
	gitRun(t, work, "push", "origin", branch)
	local := revParse(t, work, branch)
	// Origin advances without the local branch moving, so it falls behind.
	gitRun(t, work, "commit", "--allow-empty", "-m", "server-commit")
	gitRun(t, work, "push", "origin", "main:"+branch)

	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt create on a behind existing branch: %v", err)
	}

	if got := revParse(t, work, branch); got != local {
		t.Errorf("branch %s moved to %s under --yes, want unchanged %s", branch, got, local)
	}

	var got domain.CreateResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode create JSON: %v (payload %q)", err, stdout)
	}
	if got.OriginState != domain.DivergenceLabelBehind {
		t.Errorf("origin_state = %q, want %q", got.OriginState, domain.DivergenceLabelBehind)
	}
}

// LUC-172: --ff updates whichever branch the worktree actually starts from — the
// reused branch itself, not the unrelated --from parent (ffSubject).
func TestWtCreateFFRetargetsReusedBranch(t *testing.T) {
	work := repoWithRemote(t)
	stateDir := filepath.Join(work, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, work)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/behind"
	gitRun(t, work, "branch", branch)
	gitRun(t, work, "push", "origin", branch)
	gitRun(t, work, "commit", "--allow-empty", "-m", "server-commit")
	gitRun(t, work, "push", "origin", "main:"+branch)
	wantHead := revParse(t, work, "origin/"+branch)
	mainBefore := revParse(t, work, "main")

	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--"+domain.FlagFF,
		"--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create --ff on a behind existing branch: %v", err)
	}

	if got := revParse(t, work, branch); got != wantHead {
		t.Errorf("branch %s = %s, want fast-forwarded to origin %s", branch, got, wantHead)
	}
	if got := revParse(t, work, "main"); got != mainBefore {
		t.Errorf("--ff moved main (%s) instead of the reused branch, want unchanged %s", got, mainBefore)
	}
}

// repoWithRemote returns a work repo (on main) wired to a fresh bare origin, with
// main pushed so origin-tracking refs exist.
func repoWithRemote(t *testing.T) string {
	t.Helper()
	work := gittest.InitRepo(t)
	origin := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}
	gitRun(t, work, "remote", "add", "origin", origin)
	gitRun(t, work, "push", "-u", "origin", "main")
	return work
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}

// gitCommitOn adds an empty commit on branch without checking it out, so the
// branch tip differs from the base branch.
func gitCommitOn(t *testing.T, dir, branch, message string) {
	t.Helper()
	gitRun(t, dir, "commit", "--allow-empty", "-m", message)
	gitRun(t, dir, "branch", "-f", branch, "HEAD")
	gitRun(t, dir, "reset", "--hard", "HEAD~1")
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s in %s: %v", ref, dir, err)
	}
	return strings.TrimSpace(string(out))
}
