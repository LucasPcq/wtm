package wt

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/ghtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// newerPRs is a repository busier than any listing window: more pull requests
// than the hundred newest a listing used to read, none of them for a worktree.
func newerPRs(count int) []ghtest.PR {
	prs := make([]ghtest.PR, 0, count)
	for index := range count {
		prs = append(prs, ghtest.PR{Number: 10000 - index, Branch: fmt.Sprintf("bulk/topic-%d", index), State: domain.PRStateMerged})
	}
	return prs
}

// LUC-269: prune listed the hundred newest pull requests of the repository, so a
// worktree whose pull request was older was never seen as merged.
func TestPruneMatchesAWorktreeWhosePRIsOlderThanTheHundredNewest(t *testing.T) {
	setupPrune(t, pruneSetup{Worktrees: []string{"old-merged-wt", "fresh-wt"}})
	ghtest.Stub(t, ghtest.StubParams{PRs: append(newerPRs(150),
		ghtest.PR{Number: 1, Branch: "old-merged-wt", State: domain.PRStateMerged},
	)})

	result, _ := runPruneJSON(t, "--"+domain.FlagDryRun)

	assertSameBranches(t, prunedBranches(result), []string{"old-merged-wt"})
}

// LUC-269: `tree --with-prs` read the same hundred-newest listing.
func TestTreeWithPRsShowsAPROlderThanTheHundredNewest(t *testing.T) {
	setupPrune(t, pruneSetup{Worktrees: []string{"old-merged-wt"}})
	ghtest.Stub(t, ghtest.StubParams{PRs: append(newerPRs(150),
		ghtest.PR{Number: 1, Branch: "old-merged-wt", State: domain.PRStateMerged},
	)})

	stdout, stderr, err := runWtCmd(t, domain.CmdTree, "--"+domain.FlagWithPRs, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("tree: %v\nstderr: %s", err, stderr)
	}
	var forest domain.Forest
	if jsonErr := json.Unmarshal([]byte(stdout), &forest); jsonErr != nil {
		t.Fatalf("tree stdout is not JSON: %v\n%s", jsonErr, stdout)
	}
	node, found := findTreeNode(forest.Roots, "old-merged-wt")
	if !found {
		t.Fatalf("old-merged-wt missing from the tree: %s", stdout)
	}
	if node.Status.PR == nil || node.Status.PR.Number != 1 || node.Status.PR.State != domain.PRStateMerged {
		t.Errorf("old-merged-wt PR = %+v, want #1 merged", node.Status.PR)
	}
}

func findTreeNode(nodes []domain.TreeNode, branch string) (domain.TreeNode, bool) {
	for _, node := range nodes {
		if node.Branch == branch {
			return node, true
		}
		if found, ok := findTreeNode(node.Children, branch); ok {
			return found, true
		}
	}
	return domain.TreeNode{}, false
}

// LUC-269: prune's fetch reads only the branches it can prune, so a repository
// with thousands of other branches costs it nothing. A branch without a worktree
// keeps its remote-tracking ref as it was; a worktree's is brought up to date.
func TestPruneFetchesOnlyTheWorktreeBranches(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"live-wt"}})
	gittest.CreateBranch(t, repo.dir, "no-worktree")
	gittest.Git(t, repo.dir, "push", "origin", "no-worktree")
	gittest.DeleteBranchInRemote(t, repo.remote, "no-worktree")
	advanced := advanceInRemote(t, repo.remote, "live-wt")
	ghtest.Stub(t, ghtest.StubParams{})

	runPruneJSON(t, "--"+domain.FlagDryRun)

	if !refExists(repo.dir, "refs/remotes/origin/no-worktree") {
		t.Error("origin/no-worktree was pruned, want it left as it was (no worktree)")
	}
	if got := strings.TrimSpace(gitOutput(t, repo.dir, "rev-parse", "refs/remotes/origin/live-wt")); got != advanced {
		t.Errorf("origin/live-wt = %s, want the remote's new tip %s", got, advanced)
	}
}

// LUC-269: a branch tracking a remote branch of another name is gone when that
// remote branch is, as `git fetch --prune` reported it.
func TestPruneGoneFollowsAnUpstreamOfAnotherName(t *testing.T) {
	repo := setupPrune(t, pruneSetup{Worktrees: []string{"local-name"}})
	gittest.Git(t, repo.paths["local-name"], "push", "-u", "origin", "local-name:remote-name")
	gittest.DeleteBranchInRemote(t, repo.remote, "remote-name")
	ghtest.Stub(t, ghtest.StubParams{})

	result, _ := runPruneJSON(t, "--"+domain.FlagGone, "--"+domain.FlagDryRun)

	assertSameBranches(t, prunedBranches(result), []string{"local-name"})
}

// advanceInRemote commits on a branch inside the bare repository, as a push
// from elsewhere would, and returns the new tip.
func advanceInRemote(t *testing.T, remote, branch string) string {
	t.Helper()
	tree := strings.TrimSpace(gitOutput(t, remote, "rev-parse", branch+"^{tree}"))
	cmd := exec.Command("git", "commit-tree", tree, "-p", branch, "-m", "pushed from elsewhere")
	cmd.Dir = remote
	cmd.Env = append(cmd.Environ(), "GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=x@x", "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=x@x")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("commit-tree: %v", err)
	}
	tip := strings.TrimSpace(string(out))
	gittest.Git(t, remote, "update-ref", "refs/heads/"+branch, tip)
	return tip
}

func refExists(dir, ref string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref)
	cmd.Dir = dir
	return cmd.Run() == nil
}
