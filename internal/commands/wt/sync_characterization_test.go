package wt

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// These tests characterize `wtm sync` as it behaves today, before the flow/
// migration: which worktrees each entry selects, what --all, --dry-run,
// --push/--no-push and --yes resolve to, and the order of the cascade. They
// must pass unchanged after the migration.

type syncSetup struct {
	// Stack describes the worktrees to create, in order: each entry is
	// "branch:parent". The parent must already exist.
	Stack []string
}

type syncRepo struct {
	dir      string
	stateDir string
	remote   string
	paths    map[string]string
}

func setupSync(t *testing.T, setup syncSetup) syncRepo {
	t.Helper()
	repo := syncRepo{paths: map[string]string{}}
	repo.dir = gittest.InitRepo(t)
	repo.stateDir = filepath.Join(repo.dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, repo.dir)
	t.Setenv(domain.EnvStateDir, repo.stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := writeSyncConfig(repo.stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	repo.remote = gittest.AddOrigin(t, repo.dir)

	for _, entry := range setup.Stack {
		branch, parent, ok := strings.Cut(entry, ":")
		if !ok {
			t.Fatalf("stack entry %q must be \"branch:parent\"", entry)
		}
		repo.create(t, branch, parent)
	}
	return repo
}

func writeSyncConfig(stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	content := `[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = []
on_clean = []
`
	return os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644)
}

func (r *syncRepo) create(t *testing.T, branch, from string) {
	t.Helper()
	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch,
		"--from", from, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	var created struct {
		Path string `json:"path"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &created); jsonErr != nil || created.Path == "" {
		t.Fatalf("create %s: cannot read the worktree path from %q", branch, stdout)
	}
	r.paths[branch] = created.Path
	gittest.PushBranch(t, r.dir, branch)
}

// commitOn adds a commit to a branch, in its worktree (or the main repo for the
// base), and publishes it when push is requested.
func (r syncRepo) commitOn(t *testing.T, branch, file string, push bool) {
	t.Helper()
	dir := r.paths[branch]
	if dir == "" {
		dir = r.dir
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(file+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	gittest.Git(t, dir, "add", file)
	gittest.Git(t, dir, "commit", "-m", "add "+file)
	if push {
		gittest.PushBranch(t, dir, branch)
	}
}

// commitContent writes an explicit content to a file on a branch and commits it,
// unlike commitOn (task 1) whose content is derived from the file name alone —
// two commitOn calls on the same file are byte-identical (same tree, same
// parent, same message) and git collapses them into one object, so they can
// never conflict. commitContent lets two branches diverge on the same line.
func (r syncRepo) commitContent(t *testing.T, branch, file, content string, push bool) {
	t.Helper()
	dir := r.paths[branch]
	if dir == "" {
		dir = r.dir
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	gittest.Git(t, dir, "add", file)
	gittest.Git(t, dir, "commit", "-m", "edit "+file)
	if push {
		gittest.PushBranch(t, dir, branch)
	}
}

func (r syncRepo) tipOf(t *testing.T, branch string) string {
	t.Helper()
	dir := r.paths[branch]
	if dir == "" {
		dir = r.dir
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", branch).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", branch, err)
	}
	return strings.TrimSpace(string(out))
}

// syncJSON runs sync in machine output and returns the decoded result.
func syncJSON(t *testing.T, args ...string) domain.SyncResult {
	t.Helper()
	full := append([]string{domain.CmdSync}, args...)
	full = append(full, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	stdout, _, err := runWtCmd(t, full...)
	if err != nil {
		t.Fatalf("sync %v: %v", args, err)
	}
	var result domain.SyncResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("sync %v: cannot decode %q: %v", args, stdout, jsonErr)
	}
	return result
}

// syncJSONAllowFailure is syncJSON's counterpart for a cascade that is
// expected to conflict: rules.HasSyncFailure makes the command return
// domain.ErrAborted after it has already written a valid SyncResult to
// stdout (see the comment on domain.ErrAborted), so plain syncJSON's
// "no error" check does not fit here. It requires that exact error rather
// than merely tolerating it, so a refactor that stopped signalling a
// conflict through the exit code — an observable CLI behavior — fails this
// suite instead of passing silently.
func syncJSONAllowFailure(t *testing.T, args ...string) domain.SyncResult {
	t.Helper()
	full := append([]string{domain.CmdSync}, args...)
	full = append(full, "--"+domain.FlagOutput, domain.OutputJSON, "--"+domain.FlagYes)
	stdout, _, err := runWtCmd(t, full...)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("sync %v: want %v, got %v", args, domain.ErrAborted, err)
	}
	return decodeSoleSyncJSON(t, stdout)
}

// decodeSoleSyncJSON decodes the one domain.SyncResult a run wrote to stdout
// and asserts nothing else follows it. Unlike the production root command,
// runWtCmd's ad hoc root (integration_test.go) does not set SilenceUsage, so
// cobra appends a usage block to stdout after a returned error; that known,
// named block is the only trailing content tolerated — anything else,
// including a second JSON value, fails the assertion.
func decodeSoleSyncJSON(t *testing.T, stdout string) domain.SyncResult {
	t.Helper()
	reader := strings.NewReader(stdout)
	dec := json.NewDecoder(reader)
	var result domain.SyncResult
	if err := dec.Decode(&result); err != nil {
		t.Fatalf("cannot decode %q: %v", stdout, err)
	}
	rest, err := io.ReadAll(io.MultiReader(dec.Buffered(), reader))
	if err != nil {
		t.Fatalf("cannot read trailing stdout: %v", err)
	}
	before, _, _ := strings.Cut(strings.TrimSpace(string(rest)), "Usage:")
	if strings.TrimSpace(before) != "" {
		t.Fatalf("stdout carries more than one JSON value: %q", stdout)
	}
	return result
}

func TestSyncArgSelectsOnlyThatWorktree(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "feat-a")

	if len(result.Steps) != 1 || result.Steps[0].Branch != "feat-a" {
		t.Fatalf("sync feat-a must rebase feat-a alone, got %+v", result.Steps)
	}
}

func TestSyncAllSyncsEveryWorktree(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	if len(result.Steps) != 2 {
		t.Fatalf("--all must rebase every worktree, got %+v", result.Steps)
	}
}

func TestSyncAllRefusesBranchArguments(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "feat-a", "--"+domain.FlagAll, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--all combined with branch arguments must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestSyncPushAndNoPushAreMutuallyExclusive(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagPush, "--"+domain.FlagNoPush, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--push and --no-push must be refused together")
	}
}

func TestSyncFFParentsAndNoFFParentsAreMutuallyExclusive(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagFFParents, "--"+domain.FlagNoFFParents, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--ff-parents and --no-ff-parents must be refused together")
	}
}

func TestSyncYesWithoutTargetNamesAll(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--yes with no branch and no --all must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestSyncJSONRequiresYesOrDryRun(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--output", domain.OutputJSON)
	if err == nil {
		t.Fatal("--output json without --yes or --dry-run must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("the refusal must name --yes, got: %v", err)
	}
}

func TestSyncDryRunRebasesNothing(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)
	before := repo.tipOf(t, "feat-a")

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagDryRun, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("sync --dry-run: %v", err)
	}
	if after := repo.tipOf(t, "feat-a"); after != before {
		t.Fatalf("--dry-run moved feat-a: %s → %s", before, after)
	}
}

func TestSyncYesDoesNotPushByDefault(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	for _, step := range result.Steps {
		if step.Pushed {
			t.Fatalf("--yes must not push; %s was pushed", step.Branch)
		}
	}
}

func TestSyncPushForcePushesRebasedBranches(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll, "--"+domain.FlagPush)

	pushed := false
	for _, step := range result.Steps {
		if step.Pushed {
			pushed = true
		}
	}
	if !pushed {
		t.Fatalf("--push must push the rebased branches, got %+v", result.Steps)
	}
}

func TestSyncNoPushNeverPushes(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll, "--"+domain.FlagNoPush)

	for _, step := range result.Steps {
		if step.Pushed {
			t.Fatalf("--no-push must never push; %s was pushed", step.Branch)
		}
	}
}

func TestSyncCascadeKeepsTopologicalOrder(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a", "feat-c:feat-b"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	order := make([]string, 0, len(result.Steps))
	for _, step := range result.Steps {
		order = append(order, step.Branch)
	}
	want := []string{"feat-a", "feat-b", "feat-c"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("cascade order = %v, want %v (parents before children)", order, want)
	}
}

// A conflict skips the SELECTED descendants of the offending node, not every
// branch: an independent branch keeps syncing.
func TestSyncConflictSkipsSelectedDescendantsOnly(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: nil})
	repo.commitContent(t, "main", "shared.txt", "line one\n", true)
	repo.create(t, "feat-a", "main")
	repo.create(t, "feat-b", "feat-a")
	repo.create(t, "solo", "main")
	soloBefore := repo.tipOf(t, "solo")

	// Both sides then edit the SAME line differently: a real conflict, not two
	// identical commits that git would collapse into the same object.
	repo.commitContent(t, "main", "shared.txt", "line one from main\n", true)
	repo.commitContent(t, "feat-a", "shared.txt", "line one from feat-a\n", false)

	result := syncJSONAllowFailure(t, "--"+domain.FlagAll)

	byBranch := map[string]domain.SyncStepResult{}
	for _, step := range result.Steps {
		byBranch[step.Branch] = step
	}
	if byBranch["feat-a"].Status != domain.SyncStatusConflict {
		t.Fatalf("feat-a must conflict, got %+v", byBranch["feat-a"])
	}
	if byBranch["feat-b"].Status != domain.SyncStatusSkippedAncestor {
		t.Fatalf("feat-b (descendant of the conflict) must be skipped, got %+v", byBranch["feat-b"])
	}
	if repo.tipOf(t, "solo") == soloBefore {
		t.Fatal("an independent branch must keep syncing through another branch's conflict")
	}

	// solo must actually run after feat-a for the assertion above to prove
	// anything: were it ordered first, its tip would move regardless of
	// whether the cascade tolerates a sibling's conflict at all.
	order := make([]string, 0, len(result.Steps))
	for _, step := range result.Steps {
		order = append(order, step.Branch)
	}
	want := []string{"feat-a", "solo", "feat-b"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("step order = %v, want %v (solo after feat-a)", order, want)
	}
}

// Without --keep-conflict the rebase is aborted: the worktree is left clean.
func TestSyncConflictAbortsAndLeavesWorktreeClean(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: nil})
	repo.commitContent(t, "main", "shared.txt", "line one\n", true)
	repo.create(t, "feat-a", "main")
	repo.commitContent(t, "main", "shared.txt", "line one from main\n", true)
	repo.commitContent(t, "feat-a", "shared.txt", "line one from feat-a\n", false)

	syncJSONAllowFailure(t, "--"+domain.FlagAll)

	if _, err := os.Stat(filepath.Join(repo.paths["feat-a"], ".git")); err != nil {
		t.Fatalf("feat-a worktree must survive the aborted rebase: %v", err)
	}
	if inRebase(t, repo.paths["feat-a"]) {
		t.Fatal("without --keep-conflict the rebase must be aborted, not left in progress")
	}
}

func TestSyncKeepConflictLeavesRebaseInProgress(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: nil})
	repo.commitContent(t, "main", "shared.txt", "line one\n", true)
	repo.create(t, "feat-a", "main")
	repo.commitContent(t, "main", "shared.txt", "line one from main\n", true)
	repo.commitContent(t, "feat-a", "shared.txt", "line one from feat-a\n", false)

	syncJSONAllowFailure(t, "--"+domain.FlagAll, "--"+domain.FlagKeepConflict)

	if !inRebase(t, repo.paths["feat-a"]) {
		t.Fatal("--keep-conflict must leave the rebase in progress in its worktree")
	}
}

// inRebase reads a worktree's rebase state through the directory git leaves
// behind.
func inRebase(t *testing.T, path string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", path, "rev-parse", "--git-path", "rebase-merge").Output()
	if err != nil {
		t.Fatalf("rev-parse --git-path: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(path, dir)
	}
	_, statErr := os.Stat(dir)
	return statErr == nil
}

// The no-picker path (--yes) prints the plan on stderr BEFORE the recap, which
// goes on stdout. This is the order the migration must reproduce byte for byte.
func TestSyncYesPrintsPlanOnStderrThenRecapOnStdout(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	stdout, stderr, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("sync --all --yes: %v", err)
	}
	if !strings.Contains(stderr, "Sync plan") {
		t.Fatalf("the plan must be printed on stderr, got: %q", stderr)
	}
	if strings.Contains(stdout, "Sync plan") {
		t.Fatalf("the plan must not reach stdout, got: %q", stdout)
	}
	if !strings.Contains(stdout, "feat-a") {
		t.Fatalf("the recap must name the synced branch on stdout, got: %q", stdout)
	}
	// The recap is preceded by exactly one blank line.
	if !strings.HasPrefix(stdout, "\n") {
		t.Fatalf("the recap must keep its single leading blank line, got: %q", stdout)
	}
	if strings.HasPrefix(stdout, "\n\n") {
		t.Fatalf("the recap must not stack two blank lines, got: %q", stdout)
	}
}

// --dry-run still prints the recap and its push-readiness summary on stdout
// (nothing is pushed, but PushPending is still reported) — only the plan on
// stderr is characterized here; see TestSyncDryRunRebasesNothing for the "no
// mutation" guarantee.
func TestSyncDryRunPrintsThePlan(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	_, stderr, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--"+domain.FlagDryRun)
	if err != nil {
		t.Fatalf("sync --dry-run: %v", err)
	}
	if !strings.Contains(stderr, "Sync plan") {
		t.Fatalf("--dry-run must print the plan, got: %q", stderr)
	}
}

// The empty-plan message only fires when the selection excludes the base: with
// --all the service always receives a nil selection, which SyncIncludesBase
// reads as "covers the whole forest including its root" — so --all alone never
// takes this path. Naming the main worktree's own branch under a --base that
// does not match it is the one selection that both empties the plan (the main
// worktree is always excluded from steps) and excludes the base.
func TestSyncNothingToSyncReportsIt(t *testing.T) {
	setupSync(t, syncSetup{Stack: nil})

	stdout, _, err := runWtCmd(t, domain.CmdSync, "main",
		"--"+domain.FlagBase, "other-base", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("sync main --base other-base: %v", err)
	}
	if !strings.Contains(stdout, "No worktrees to sync") {
		t.Fatalf("an empty plan must say so, got: %q", stdout)
	}
}
