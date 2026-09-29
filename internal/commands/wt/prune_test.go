package wt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/ghtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// These tests characterize `wtm prune` as it behaves today, before the flow/
// migration: which worktrees each reason filter matches, what --dry-run, --force
// and --yes resolve to, the order of on_clean hooks against the removals, and
// what happens with no GitHub CLI. They must pass unchanged after the migration.

type pruneSetup struct {
	// Worktrees are created off main, in order, and pushed with an upstream.
	Worktrees []string
	// CleanHook installs an on_clean hook that records, per worktree, how many
	// worktrees still existed when it ran — which is what makes the "every hook
	// before any removal" order observable.
	CleanHook bool
	// FailHookOn makes that hook exit non-zero for one branch.
	FailHookOn string
}

// pruneRepo is the fixture a prune test acts on: the main repo, the state dir,
// the bare origin, and where each created worktree landed.
type pruneRepo struct {
	dir      string
	stateDir string
	remote   string
	treesDir string
	hookLog  string
	paths    map[string]string
}

// setupPrune builds a repo with an origin remote so upstream state (and the
// "[gone]" prune reads) is reproducible, plus one worktree per branch.
func setupPrune(t *testing.T, setup pruneSetup) pruneRepo {
	t.Helper()
	repo := pruneRepo{paths: map[string]string{}}
	repo.dir = gittest.InitRepo(t)
	repo.stateDir = filepath.Join(repo.dir, ".git", "wtm")
	repo.treesDir = filepath.Join(filepath.Dir(repo.dir), ".trees")
	t.Setenv("WTM_PROJECT_DIR", repo.dir)
	t.Setenv("WTM_STATE_DIR", repo.stateDir)
	t.Setenv(domain.EnvGoFile, "")

	var onClean []string
	if setup.CleanHook {
		repo.hookLog = filepath.Join(t.TempDir(), "on-clean.log")
		onClean = []string{fmt.Sprintf("%s %s {{branch}} %s %s",
			writeCleanHookScript(t), repo.hookLog, repo.treesDir, setup.FailHookOn)}
	}
	if err := writePruneConfig(repo.stateDir, onClean); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	repo.remote = gittest.AddOrigin(t, repo.dir)

	for _, branch := range setup.Worktrees {
		repo.create(t, branch, "main")
	}
	return repo
}

// create adds one worktree off from, and publishes its branch so upstream state
// is defined for every candidate prune classifies.
func (r *pruneRepo) create(t *testing.T, branch, from string) {
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

// writeCleanHookScript emits the on_clean hook: it appends "<branch>:<worktrees
// still on disk>" to a log, then optionally fails. Hooks run through
// exec.Command on whitespace-split fields, so this has to be a real script.
func writeCleanHookScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "on-clean.sh")
	script := `#!/bin/sh
log="$1"; branch="$2"; trees="$3"; fail="$4"
count=$(ls -1 "$trees" 2>/dev/null | wc -l | tr -d ' ')
echo "$branch:$count" >> "$log"
if [ -n "$fail" ] && [ "$branch" = "$fail" ]; then exit 1; fi
exit 0
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write on_clean hook: %v", err)
	}
	return path
}

// hookRuns reads back what the on_clean hook recorded, in order.
func (r pruneRepo) hookRuns(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(r.hookLog)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read hook log: %v", err)
	}
	return strings.Fields(strings.TrimSpace(string(data)))
}

func writePruneConfig(stateDir string, onClean []string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	quoted := make([]string, 0, len(onClean))
	for _, hook := range onClean {
		quoted = append(quoted, fmt.Sprintf("%q", hook))
	}
	content := fmt.Sprintf(`[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = []
on_clean = [%s]
`, strings.Join(quoted, ", "))
	return os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644)
}

func runPruneJSON(t *testing.T, args ...string) (domain.PruneResult, string) {
	t.Helper()
	stdout, stderr, err := runWtCmd(t, append([]string{domain.CmdPrune, "--output", domain.OutputJSON}, args...)...)
	if err != nil {
		t.Fatalf("prune %v: %v\nstderr: %s", args, err, stderr)
	}
	var result domain.PruneResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("prune %v: stdout is not JSON: %v\n%s", args, jsonErr, stdout)
	}
	return result, stderr
}

func prunedBranches(result domain.PruneResult) []string {
	branches := make([]string, 0, len(result.Pruned))
	for _, candidate := range result.Pruned {
		branches = append(branches, candidate.Branch)
	}
	return branches
}

func skipReason(t *testing.T, result domain.PruneResult, branch string) string {
	t.Helper()
	for _, skip := range result.Skipped {
		if skip.Branch == branch {
			return skip.Reason
		}
	}
	return ""
}

func assertSameBranches(t *testing.T, got, want []string) {
	t.Helper()
	gotSet := strings.Join(sortedCopy(got), ",")
	wantSet := strings.Join(sortedCopy(want), ",")
	if gotSet != wantSet {
		t.Errorf("branches = [%s], want [%s]", gotSet, wantSet)
	}
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// dirty leaves an uncommitted file in a worktree, which makes it unsafe to
// remove without --force.
func (r pruneRepo) dirty(t *testing.T, branch string) {
	t.Helper()
	path := r.paths[branch]
	if path == "" {
		t.Fatalf("no recorded path for worktree %s", branch)
	}
	if err := os.WriteFile(filepath.Join(path, "scratch.txt"), []byte("uncommitted"), 0o644); err != nil {
		t.Fatalf("dirty %s: %v", branch, err)
	}
}

func (r pruneRepo) assertWorktreeExists(t *testing.T, branch string, want bool) {
	t.Helper()
	path := r.paths[branch]
	if path == "" {
		t.Fatalf("no recorded path for worktree %s", branch)
	}
	_, err := os.Stat(path)
	if want && err != nil {
		t.Errorf("worktree %s should still exist at %s: %v", branch, path, err)
	}
	if !want && err == nil {
		t.Errorf("worktree %s should have been removed, %s still exists", branch, path)
	}
}

// --- Reason filters -------------------------------------------------------

// TestPruneMergedOnlyMatchesMergedPRs: "done" is read from GitHub, never guessed
// from local commits, and --merged narrows to that one category.
func TestPruneMergedOnlyMatchesMergedPRs(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt", "closed-wt", "open-wt"}})
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "merged-wt", State: domain.PRStateMerged},
		{Number: 2, Branch: "closed-wt", State: domain.PRStateClosed},
		{Number: 3, Branch: "open-wt", State: domain.PRStateOpen},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagMerged, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"merged-wt"})
	repo.assertWorktreeExists(t, "closed-wt", true)
	repo.assertWorktreeExists(t, "open-wt", true)
}

// TestPruneClosedOnlyMatchesClosedPRs: a PR closed without merging is its own
// category — a merged one must not answer for it.
func TestPruneClosedOnlyMatchesClosedPRs(t *testing.T) {
	setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt", "closed-wt"}})
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "merged-wt", State: domain.PRStateMerged},
		{Number: 2, Branch: "closed-wt", State: domain.PRStateClosed},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagClosed, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"closed-wt"})
}

// TestPruneDefaultMatchesEveryFinishedCategory: with no reason flag prune is
// broad — merged, closed and gone all count as finished. An open PR never does.
func TestPruneDefaultMatchesEveryFinishedCategory(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt", "closed-wt", "gone-wt", "open-wt"}})
	gittest.DeleteBranchInRemote(t, repo.remote, "gone-wt")
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "merged-wt", State: domain.PRStateMerged},
		{Number: 2, Branch: "closed-wt", State: domain.PRStateClosed},
		{Number: 3, Branch: "open-wt", State: domain.PRStateOpen},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"merged-wt", "closed-wt", "gone-wt"})
}

// TestPruneGoneMatchesDeletedUpstream: --gone reads the local branch's upstream
// tracking state, which the fetch it runs first is what makes accurate.
func TestPruneGoneMatchesDeletedUpstream(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"gone-wt", "live-wt"}})
	gittest.DeleteBranchInRemote(t, repo.remote, "gone-wt")
	ghtest.Stub(t, ghtest.StubParams{})

	result, _ := runPruneJSON(t, "--"+domain.FlagGone, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"gone-wt"})
	if len(result.Pruned) == 1 && result.Pruned[0].Reason != domain.PruneReasonGone {
		t.Errorf("reason = %q, want %q", result.Pruned[0].Reason, domain.PruneReasonGone)
	}
}

// TestPruneNoFetchUsesAlreadyFetchedState: --no-fetch skips the git fetch
// --prune, so a branch deleted on the remote since the last fetch is not seen.
func TestPruneNoFetchUsesAlreadyFetchedState(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"gone-wt"}})
	gittest.DeleteBranchInRemote(t, repo.remote, "gone-wt")
	ghtest.Stub(t, ghtest.StubParams{})

	stale, _ := runPruneJSON(t, "--"+domain.FlagGone, "--"+domain.FlagNoFetch, "--"+domain.FlagYes)
	if len(stale.Pruned) != 0 {
		t.Errorf("--no-fetch pruned %v, want nothing (the stale ref still exists)", prunedBranches(stale))
	}
	repo.assertWorktreeExists(t, "gone-wt", true)

	fresh, _ := runPruneJSON(t, "--"+domain.FlagGone, "--"+domain.FlagYes)
	assertSameBranches(t, prunedBranches(fresh), []string{"gone-wt"})
}

// --- GitHub CLI availability ----------------------------------------------

// TestPruneWithoutGHOnlyGoneApplies: merged/closed detection is GitHub-truth, so
// without the CLI it is inert — and the advisory goes to stderr, never onto the
// JSON stdout a caller is parsing.
func TestPruneWithoutGHOnlyGoneApplies(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt", "gone-wt"}})
	gittest.DeleteBranchInRemote(t, repo.remote, "gone-wt")
	ghtest.Absent(t)

	result, stderr := runPruneJSON(t, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"gone-wt"})
	if !strings.Contains(stderr, "GitHub CLI") {
		t.Errorf("stderr should carry the gh advisory, got %q", stderr)
	}
}

// TestPruneUnauthenticatedGHIsAdvised: an installed but unauthenticated gh is
// reported distinctly, so the fix named is `gh auth login`, not an install.
func TestPruneUnauthenticatedGHIsAdvised(t *testing.T) {
	setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt"}})
	ghtest.Stub(t, ghtest.StubParams{Unauthenticated: true})

	result, stderr := runPruneJSON(t, "--"+domain.FlagYes)

	if len(result.Pruned) != 0 {
		t.Errorf("pruned %v with no PR data, want nothing", prunedBranches(result))
	}
	if !strings.Contains(stderr, "gh auth login") {
		t.Errorf("stderr should name `gh auth login`, got %q", stderr)
	}
}

// --- Bypass axes ----------------------------------------------------------

// TestPruneDryRunRemovesNothing: --dry-run previews and mutates nothing, and it
// is a bypass of its own — it does not need --yes, even in JSON.
func TestPruneDryRunRemovesNothing(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt"}})
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "merged-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagDryRun)

	if !result.DryRun {
		t.Error("result should be flagged as a dry run")
	}
	assertSameBranches(t, prunedBranches(result), []string{"merged-wt"})
	repo.assertWorktreeExists(t, "merged-wt", true)
}

// TestPruneJSONRequiresYesOrDryRun: the selection prompt cannot run in JSON
// mode, so the confirmation axis must be bypassed explicitly.
func TestPruneJSONRequiresYesOrDryRun(t *testing.T) {
	setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt"}})
	ghtest.Stub(t, ghtest.StubParams{})

	_, _, err := runWtCmd(t, domain.CmdPrune, "--output", domain.OutputJSON)
	if err == nil {
		t.Fatal("expected --output json without --yes or --dry-run to be rejected")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagYes)
	}
}

// TestPruneWithoutTerminalRefuses: a picker only ever runs in a fully
// interactive run, so with no terminal prune refuses rather than prompting.
func TestPruneWithoutTerminalRefuses(t *testing.T) {
	setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt"}})
	ghtest.Stub(t, ghtest.StubParams{})

	_, _, err := runWtCmd(t, domain.CmdPrune)
	if err == nil {
		t.Fatal("expected a run with no terminal to refuse rather than prompt")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagYes)
	}
}

// TestPruneYesKeepsUnsafeWorktrees: --yes is the confirmation axis, not a
// licence to remove something unsafe. A dirty worktree is skipped with its
// reason, alongside the safe ones that are removed.
func TestPruneYesKeepsUnsafeWorktrees(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"dirty-wt", "safe-wt"}})
	repo.dirty(t, "dirty-wt")
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "dirty-wt", State: domain.PRStateMerged},
		{Number: 2, Branch: "safe-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"safe-wt"})
	if reason := skipReason(t, result, "dirty-wt"); reason != domain.PruneSkipDirty {
		t.Errorf("skip reason = %q, want %q", reason, domain.PruneSkipDirty)
	}
	repo.assertWorktreeExists(t, "dirty-wt", true)
	repo.assertWorktreeExists(t, "safe-wt", false)
}

// TestPruneAllUnsafeReportsNothing pins a wart worth keeping deliberate: when
// every match is skipped, the run reports an empty result rather than the skips
// that explain it. The migration must not change that by accident — fixing it is
// a separate decision.
func TestPruneAllUnsafeReportsNothing(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"dirty-wt"}})
	repo.dirty(t, "dirty-wt")
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "dirty-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagYes)

	if len(result.Pruned) != 0 || len(result.Skipped) != 0 {
		t.Errorf("result = %+v, want an empty one", result)
	}
	repo.assertWorktreeExists(t, "dirty-wt", true)
}

// TestPruneForceRemovesUnsafeWorktrees: --force is the safety axis and lifts the
// refusal --yes deliberately kept.
func TestPruneForceRemovesUnsafeWorktrees(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"dirty-wt"}})
	repo.dirty(t, "dirty-wt")
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "dirty-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagForce, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"dirty-wt"})
	repo.assertWorktreeExists(t, "dirty-wt", false)
}

// TestPruneKeepsBaseAndMainProtected: the base branch and the main worktree are
// spared unconditionally, force included.
func TestPruneKeepsBaseAndMainProtected(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"merged-wt"}})
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "main", State: domain.PRStateMerged},
		{Number: 2, Branch: "merged-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagForce, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"merged-wt"})
	if _, err := os.Stat(repo.dir); err != nil {
		t.Errorf("the main worktree must survive: %v", err)
	}
}

// TestPruneNothingToPrune: an empty selection is reported as such, in both
// formats, without touching anything.
func TestPruneNothingToPrune(t *testing.T) {
	setupPrune(t, pruneSetup{Worktrees: []string{"live-wt"}})
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "live-wt", State: domain.PRStateOpen},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagYes)
	if len(result.Pruned) != 0 {
		t.Errorf("pruned %v, want nothing", prunedBranches(result))
	}

	stdout, _, err := runWtCmd(t, domain.CmdPrune, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("human prune: %v", err)
	}
	if !strings.Contains(stdout, "Nothing to prune") {
		t.Errorf("stdout = %q, want it to say nothing to prune", stdout)
	}
}

// --- Hooks against removals -----------------------------------------------

// TestPruneTearsEachWorktreeDownBeforeTheNext: each worktree goes through its
// whole sequence — hook, removal, data — before the next one's hook runs. The
// count each hook records is what proves it: the second hook sees the first
// worktree already gone.
func TestPruneTearsEachWorktreeDownBeforeTheNext(t *testing.T) {
	repo := setupPrune(t, pruneSetup{
		Worktrees: []string{"first-wt", "second-wt"},
		CleanHook: true,
	})
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "first-wt", State: domain.PRStateMerged},
		{Number: 2, Branch: "second-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"first-wt", "second-wt"})
	if runs := strings.Join(repo.hookRuns(t), " "); runs != "first-wt:2 second-wt:1" {
		t.Errorf("hook runs = %q, want each hook run just before its own removal", runs)
	}
}

// TestPruneFailingHookStopsAtThatWorktree: a hook that fails stops the prune
// there. The worktree before it is gone, the failing one and the ones after are
// left as they were, and the JSON says where it stopped.
func TestPruneFailingHookStopsAtThatWorktree(t *testing.T) {
	repo := setupPrune(t, pruneSetup{
		Worktrees:  []string{"first-wt", "second-wt"},
		CleanHook:  true,
		FailHookOn: "first-wt",
	})
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "first-wt", State: domain.PRStateMerged},
		{Number: 2, Branch: "second-wt", State: domain.PRStateMerged},
	}})

	stdout, _, err := runWtCmd(t, domain.CmdPrune, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected a failing on_clean hook to fail the prune")
	}

	repo.assertWorktreeExists(t, "first-wt", true)
	repo.assertWorktreeExists(t, "second-wt", true)
	if runs := repo.hookRuns(t); len(runs) != 1 {
		t.Errorf("hook runs = %v, want the run to stop at the failing hook", runs)
	}
	// The harness appends cobra's usage after a failing run; the document is
	// the first value on stdout.
	var result domain.PruneResult
	if jsonErr := json.NewDecoder(strings.NewReader(stdout)).Decode(&result); jsonErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", jsonErr, stdout)
	}
	if result.Failed == nil || result.Failed.Branch != "first-wt" || len(result.Pruned) != 0 {
		t.Errorf("result = %+v, want first-wt named as where it stopped", result)
	}
}

// --- Surviving children ----------------------------------------------------

// TestPruneLeavesChildrenOrphanedByDefault: reparenting rewrites metadata, so it
// needs explicit consent. A prompt-free run without the flag leaves the child
// pointing at the branch that is gone.
func TestPruneLeavesChildrenOrphanedByDefault(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"parent-wt"}})
	repo.create(t, "child-wt", "parent-wt")
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "parent-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"parent-wt"})
	if len(result.Reparented) != 0 {
		t.Errorf("reparented %+v, want none without --%s", result.Reparented, domain.FlagReparentChildren)
	}
	if len(result.Orphaned) != 1 || result.Orphaned[0].Branch != "child-wt" {
		t.Errorf("orphaned = %+v, want child-wt", result.Orphaned)
	}
	if parent := readSourceBranch(t, repo.stateDir, "child-wt"); parent != "parent-wt" {
		t.Errorf("child parent = %q, want it left untouched at parent-wt", parent)
	}
}

// TestPruneReparentChildrenMovesThemToTheGrandparent: the flag is the consent,
// and the target is the grandparent — here the base branch the parent sat on.
func TestPruneReparentChildrenMovesThemToTheGrandparent(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"parent-wt"}})
	repo.create(t, "child-wt", "parent-wt")
	ghtest.Stub(t, ghtest.StubParams{PRs: []ghtest.PR{
		{Number: 1, Branch: "parent-wt", State: domain.PRStateMerged},
	}})

	result, _ := runPruneJSON(t, "--"+domain.FlagReparentChildren, "--"+domain.FlagYes)

	assertSameBranches(t, prunedBranches(result), []string{"parent-wt"})
	if len(result.Orphaned) != 0 {
		t.Errorf("orphaned %+v, want none with --%s", result.Orphaned, domain.FlagReparentChildren)
	}
	if len(result.Reparented) != 1 || result.Reparented[0].NewParent != "main" {
		t.Errorf("reparented = %+v, want child-wt onto main", result.Reparented)
	}
	if parent := readSourceBranch(t, repo.stateDir, "child-wt"); parent != "main" {
		t.Errorf("child parent = %q, want main", parent)
	}
}
