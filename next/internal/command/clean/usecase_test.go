package clean_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARemovalTakesTheWorktreeItsBranchAndItsRecord(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), removing(plain), domain.RunOptions{})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, map[string]domain.Status{plain: domain.StatusDone}, statuses(outcome.Result.Items))
	assert.Equal(t, &clean.Detail{Path: w.at(plain)}, outcome.Result.Items[0].Detail)
	assert.True(t, w.gone(t, plain))
	assert.True(t, w.stands(t, parent))
}

func TestEachRemovalIsReportedRemovedAndFinishedWithItsStatus(t *testing.T) {
	w := newWorld(t, seed{})

	_, report := w.apply(t, removing(plain))

	removed := eventsOf[domain.WorktreeRemoved](report.All())
	require.Len(t, removed, 1)
	assert.Equal(t, worktree.Identity{Branch: plain, Path: w.at(plain), Parent: "main", Isolation: "isolated", CreatedAt: "2026-10-25T12:00:00Z"}, removed[0].Worktree)
	assert.Equal(t, []domain.UnitFinished{{Subject: plain, Status: domain.StatusDone}}, eventsOf[domain.UnitFinished](report.All()))
}

func TestAnUnsafeWorktreeRefusesTheWholeRequestBeforeAnythingIsRemoved(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := w.usecase.Run(t.Context(), removing(plain, dirty, locked), domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, domain.KindRefused, refused.Kind)
	assert.Equal(t, clean.CodeUnsafe, refused.Code)
	assert.Equal(t, map[string][]string{"worktree[1]": {"clean.dirty"}, "worktree[2]": {"clean.locked"}}, fieldCodes(refused.Fields))
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, clean.CodeForce, refused.FollowUp.Code)
	assert.Contains(t, refused.FollowUp.Answers, domain.Answered{Path: clean.PathForce, Values: []string{"true"}})
	for _, name := range []string{plain, dirty, locked} {
		assert.True(t, w.stands(t, name), name)
	}
}

func TestForceRemovesTheUnsafeWorktrees(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), clean.Request{Worktrees: []string{dirty, locked}, Force: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, map[string]domain.Status{dirty: domain.StatusDone, locked: domain.StatusDone}, statuses(outcome.Result.Items))
	assert.True(t, w.gone(t, dirty))
	assert.True(t, w.gone(t, locked))
}

// Legacy counted origin/B..B, none for a branch origin never had: it ran the hooks, removed the worktree, then git branch -d refused, leaving the branch and its record behind.
func TestABranchHoldingCommitsNoOtherOneHoldsIsUnsafeBeforeAnythingRuns(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("touch ran")})

	_, err := w.usecase.Run(t.Context(), removing(local), domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, map[string][]string{"worktree[0]": {"clean.unpushed"}}, fieldCodes(refused.Fields))
	assert.Equal(t, "1", refused.Fields[0].Params["count"])
	assert.True(t, w.stands(t, local))
	assert.NoFileExists(t, filepath.Join(w.at(local), "ran"))
}

// Each branch counted only what no other branch held: a parent and the child made from it, removed together, each held the other's commits, and both went with them.
func TestCommitsTheBranchesRemovedTogetherHoldAreUnpushedToo(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "-f", "feat/stacked", local)
	gittest.Git(t, w.main, "worktree", "add", "-q", w.at("feat/stacked"), "feat/stacked")

	_, err := w.usecase.Run(t.Context(), removing(local, "feat/stacked"), domain.RunOptions{DryRun: true})

	assert.Equal(t, map[string][]string{"worktree[0]": {"clean.unpushed"}, "worktree[1]": {"clean.unpushed"}}, fieldCodes(failureOf(t, err).Fields))
}

// Only a branch's commits were counted: a commit made on no branch, or by a rebase under way, went with the worktree, unforced.
func TestCommitsOnlyAWorktreesHeadHoldsAreUnpushed(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.at("detached"), "commit", "-q", "--allow-empty", "-m", "on no branch")
	gittest.Git(t, w.at(plain), "checkout", "-q", "--detach")
	gittest.Git(t, w.at(plain), "commit", "-q", "--allow-empty", "-m", "picked by the rebase")
	writeFile(t, filepath.Join(gittest.Output(t, w.at(plain), "rev-parse", "--absolute-git-dir"), "rebase-merge", "head-name"), "refs/heads/"+plain+"\n")

	_, err := w.usecase.Run(t.Context(), removing(w.at("detached"), plain), domain.RunOptions{DryRun: true})

	refused := failureOf(t, err)
	assert.Equal(t, map[string][]string{"worktree[0]": {"clean.unpushed"}, "worktree[1]": {"clean.unpushed"}}, fieldCodes(refused.Fields))
	assert.Equal(t, "1", refused.Fields[1].Params["count"])
}

// HEAD was read in the worktree's directory: once it was gone, status's own fix, `clean <path>`, lost the commit only it held.
func TestCommitsOnlyAGoneWorktreesHeadHoldsAreUnpushed(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.at("detached"), "commit", "-q", "--allow-empty", "-m", "on no branch")
	gittest.Git(t, w.at(plain), "checkout", "-q", "--detach")
	gittest.Git(t, w.at(plain), "commit", "-q", "--allow-empty", "-m", "picked by the rebase")
	writeFile(t, filepath.Join(gittest.Output(t, w.at(plain), "rev-parse", "--absolute-git-dir"), "rebase-merge", "head-name"), "refs/heads/"+plain+"\n")
	require.NoError(t, os.RemoveAll(w.at("detached")))
	require.NoError(t, os.RemoveAll(w.at(plain)))

	_, err := w.usecase.Run(t.Context(), removing(w.at("detached"), w.at(plain)), domain.RunOptions{DryRun: true})

	assert.Equal(t, map[string][]string{"worktree[0]": {"clean.unpushed"}, "worktree[1]": {"clean.unpushed"}}, fieldCodes(failureOf(t, err).Fields))
}

// rev-list refused the branch of a worktree added with --orphan, which has no commit yet: it could not be removed, forced or not.
func TestAWorktreeOnABranchWithNoCommitYetIsRemoved(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "worktree", "add", "-q", "--orphan", "-b", "feat/unborn", w.at("feat/unborn"))

	result, _ := w.apply(t, removing("feat/unborn"))

	assert.Equal(t, map[string]domain.Status{"feat/unborn": domain.StatusDone}, statuses(result.Items))
	assert.NoDirExists(t, w.at("feat/unborn"))
}

// git never calls a locked worktree prunable: its directory gone (a drive unmounted), its changes were read there and failed, forced or not.
func TestALockedWorktreeWhoseDirectoryIsGoneIsRemovedForced(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.RemoveAll(w.at(locked)))

	result, _ := w.apply(t, clean.Request{Worktrees: []string{locked}, Force: true})

	assert.Equal(t, map[string]domain.Status{locked: domain.StatusDone}, statuses(result.Items))
	assert.True(t, w.gone(t, locked))
}

func TestOneWorktreeAskedByItsBranchAndItsPathIsAskedTwice(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := w.usecase.Run(t.Context(), removing(plain, w.at(plain)), domain.RunOptions{DryRun: true})

	assert.Equal(t, map[string][]string{"worktree[1]": {domain.CodeDistinct}}, fieldCodes(failureOf(t, err).Fields))
}

// Legacy deleted the branch with git branch -d, which refuses one pushed but not merged locally: the worktree was gone, its branch and record left behind.
func TestABranchOriginHoldsIsRemovedWholeThoughNotMergedLocally(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), removing(pushed), domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, map[string]domain.Status{pushed: domain.StatusDone}, statuses(outcome.Result.Items))
	assert.True(t, w.gone(t, pushed))
}

func TestABranchThatCannotBeDeletedLeavesItsWorktreeRemovedAndTheItemIncomplete(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Git = failingDelete{w.repo}

	result, report := w.apply(t, removing(plain))

	item := result.Items[0]
	assert.Equal(t, domain.StatusFailed, item.Status)
	assert.Equal(t, domain.ReasonIncomplete, item.Reason)
	assert.Equal(t, "clean.branch_kept", item.Err.Code)
	assert.NoDirExists(t, w.at(plain))
	assert.Len(t, eventsOf[domain.WorktreeRemoved](report.All()), 1)
}

func TestFilesLeftOnDiskAreNamedAndTheWorktreeCountsAsRemoved(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Git = leavingFiles{w.repo}

	result, _ := w.apply(t, removing(plain))

	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Equal(t, []string{"clean.left_on_disk"}, codes(result.Items[0].Warnings))
	assert.Contains(t, result.Items[0].Warnings[0].Message, "sudo rm -rf "+w.at(plain))
}

// A worktree git failed to remove read as removed: its children were moved off a parent that stays, warned of as orphaned, and the caller standing in it told to step out.
func TestAWorktreeGitFailsToRemoveIsNotReportedRemoved(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Git = failingRemove{w.repo, w.at(parent)}

	result, report := w.apply(t, removing(parent).From(domain.Caller{Cwd: w.at(parent)}))

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Nil(t, result.Items[0].Detail)
	assert.Empty(t, result.Warnings)
	assert.Empty(t, result.FollowUps)
	assert.Empty(t, eventsOf[domain.WorktreeRemoved](report.All()))
}

// The nearest worktree that stays was taken from the plan: a grandparent that failed to go was skipped all the same, its grandchild moved past it.
func TestAChildMovesOntoTheNearestWorktreeThatActuallyStayed(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Git = failingRemove{w.repo, w.at(parent)}

	result, _ := w.apply(t, clean.Request{Worktrees: []string{parent, child}, ReparentChildren: true})

	assert.Equal(t, map[string]domain.Status{parent: domain.StatusFailed, child: domain.StatusDone}, statuses(result.Items))
	record, err := w.meta.Load(grandchild)
	require.NoError(t, err)
	assert.Equal(t, parent, record.SourceBranch)
}

func TestTheChildrenLeftBehindAreNamedWithTheReparentThatMovesThem(t *testing.T) {
	w := newWorld(t, seed{})

	result, _ := w.apply(t, removing(parent))

	require.Equal(t, []string{"clean.orphaned"}, codes(result.Warnings))
	orphan := result.Warnings[0]
	assert.Equal(t, domain.Params{"branch": child, "from": parent, "to": "main"}, orphan.Params)
	require.NotNil(t, orphan.FollowUp)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{child}}, {Path: "to", Values: []string{"main"}}}, orphan.FollowUp.Answers)
	record, err := w.meta.Load(child)
	require.NoError(t, err)
	assert.Equal(t, parent, record.SourceBranch)
}

func TestReparentingMovesEachChildOntoTheNearestWorktreeThatStays(t *testing.T) {
	w := newWorld(t, seed{})

	result, report := w.apply(t, clean.Request{Worktrees: []string{parent, child}, ReparentChildren: true})

	assert.Empty(t, result.Warnings)
	record, err := w.meta.Load(grandchild)
	require.NoError(t, err)
	assert.Equal(t, "main", record.SourceBranch)
	updated := eventsOf[domain.WorktreeUpdated](report.All())
	require.Len(t, updated, 1)
	assert.Equal(t, grandchild, updated[0].Worktree.Branch)
	assert.Equal(t, "main", updated[0].Worktree.Parent)
	assert.Equal(t, []worktree.IdentityField{worktree.IdentityParent}, updated[0].Changed)
}

func TestAChildOfAParentThatFailedToGoIsNotOrphaned(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("exit 3")})

	result, _ := w.apply(t, removing(parent))

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Empty(t, result.Warnings)
}

func TestAFailingHookKeepsTheWorktreeAndStopsNoOther(t *testing.T) {
	w := newWorld(t, seed{Config: withHook(`[ "{{branch}}" != "feat/plain" ]`)})

	result, report := w.apply(t, removing(plain, pushed))

	assert.Equal(t, map[string]domain.Status{plain: domain.StatusFailed, pushed: domain.StatusDone}, statuses(result.Items))
	assert.Equal(t, "clean.hook_failed", result.Items[0].Err.Code)
	assert.True(t, w.stands(t, plain))
	assert.True(t, w.gone(t, pushed))
	assert.Contains(t, eventsOf[domain.PhaseFinished](report.All()), domain.PhaseFinished{Subject: plain, Phase: domain.PhaseOnClean, Failed: true})
}

func TestTheHooksRunInTheWorktreeTheirOutputReportedAndLogged(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("echo bye {{branch}} from $(basename $PWD)")})

	_, report := w.apply(t, removing(plain))

	assert.Contains(t, eventsOf[domain.Output](report.All()), domain.Output{Subject: plain, Line: "bye feat/plain from wt-feat-plain"})
	started := eventsOf[domain.PhaseStarted](report.All())
	require.NotEmpty(t, started)
	assert.Equal(t, filepath.Join(w.stateDir, "hooks", "on_clean-feat%2Fplain.log"), started[0].Log)
	assert.FileExists(t, started[0].Log)
}

func TestAWorktreeWhoseDirectoryIsGoneIsRemovedWithoutItsHooks(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("exit 3")})
	require.NoError(t, os.RemoveAll(w.at(plain)))

	result, _ := w.apply(t, removing(plain))

	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Equal(t, []string{"clean.hooks_skipped"}, codes(result.Items[0].Warnings))
	assert.True(t, w.gone(t, plain))
}

// create's undo can remove a worktree and fail to delete its branch, then offers clean, forced: legacy said the branch was already absent and kept it.
func TestABranchNoWorktreeHoldsIsDeletedWithItsRecordBehindTheSameChecks(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "feat/left", "main")
	w.record(t, "feat/left", worktree.Meta{SourceBranch: "main"})
	gittest.Git(t, w.main, "branch", "feat/held", gittest.Output(t, w.main, "commit-tree", "main^{tree}", "-p", "main", "-m", "held"))

	result, report := w.apply(t, removing("feat/left"))
	_, refused := w.usecase.Run(t.Context(), removing("feat/held"), domain.RunOptions{DryRun: true})
	forced, err := w.usecase.Run(t.Context(), clean.Request{Worktrees: []string{"feat/held"}, Force: true}, domain.RunOptions{})

	assert.Equal(t, map[string]domain.Status{"feat/left": domain.StatusDone}, statuses(result.Items))
	assert.Empty(t, gittest.Output(t, w.main, "branch", "--list", "feat/left"))
	_, metaErr := w.meta.Load("feat/left")
	assert.ErrorIs(t, metaErr, os.ErrNotExist)
	assert.Empty(t, eventsOf[domain.WorktreeRemoved](report.All()), "no worktree went")
	assert.Equal(t, map[string][]string{"worktree[0]": {"clean.unpushed"}}, fieldCodes(failureOf(t, refused).Fields))
	require.NoError(t, err)
	assert.Equal(t, map[string]domain.Status{"feat/held": domain.StatusDone}, statuses(forced.Result.Items))
	assert.Empty(t, gittest.Output(t, w.main, "branch", "--list", "feat/held"))
}

// The proof's hand run caught it: a branch alone was refused as "1 worktree(s)" with "1 commit(s)", and forced, its recap lost that it was a branch alone.
func TestARefusalNamesWhatItRefusesAndAForcedBranchAloneStaysOne(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "feat/held", gittest.Output(t, w.main, "commit-tree", "main^{tree}", "-p", "main", "-m", "held"))

	_, err := w.usecase.Run(t.Context(), removing("feat/held", dirty), domain.RunOptions{DryRun: true})
	forced, forcedErr := w.usecase.Run(t.Context(), clean.Request{Worktrees: []string{"feat/held"}, Force: true}, domain.RunOptions{DryRun: true})

	refused := failureOf(t, err)
	assert.Equal(t, "nothing removed: feat/held, "+dirty+" can't be removed safely", refused.Message)
	assert.Equal(t, "feat/held has 1 commit no other branch nor remote holds", refused.Fields[0].Message("worktree"))
	require.NoError(t, forcedErr)
	assert.Equal(t, "feat/held (branch only: no worktree; unpushed, forced)", forced.Plan.Recap[0].Values[0].Text)
}

// The base branch, held by no worktree, read as a branch to delete: `clean main` deleted it, its children moved from main onto main.
func TestTheBaseBranchIsNeverRemoved(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "checkout", "-q", "-b", "trunk")

	_, err := w.usecase.Run(t.Context(), clean.Request{Worktrees: []string{"main"}, Force: true}, domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, domain.KindInvalid, refused.Kind)
	assert.Equal(t, map[string][]string{"worktree[0]": {clean.CodeBaseBranch}}, fieldCodes(refused.Fields))
	assert.NotEmpty(t, gittest.Output(t, w.main, "branch", "--list", "main"))
}

func TestANameThatIsNoWorktreeIsAlreadyAbsent(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), removing("feat/nope", plain), domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/nope"}, outcome.Plan.Absent)
	assert.Equal(t, clean.Item{Subject: "feat/nope", Status: domain.StatusDone, Reason: domain.ReasonUnchanged}, outcome.Result.Items[1])
	assert.Nil(t, outcome.Result.Failure())
}

// go and status read a part of a branch as its name; a removal never does: run again once feat/plain went, it would take another worktree.
func TestAPartOfABranchNamesNoWorktreeToRemove(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), removing("feat/pla"), domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{"feat/pla"}, outcome.Plan.Absent)
	assert.True(t, w.stands(t, plain))
}

// A path inside a worktree named the worktree holding it: create's `→ wtm clean docs --force`, run from feat/a's root where every worktree has docs/, removed feat/a.
func TestAPathInsideAWorktreeNamesNothing(t *testing.T) {
	w := newWorld(t, seed{})
	writeFile(t, filepath.Join(w.at(plain), "docs", "keep.txt"), "x")
	writeFile(t, filepath.Join(w.at(plain), "apps", "src", "keep.txt"), "x")
	gittest.Git(t, w.at(plain), "add", ".")
	gittest.Git(t, w.at(plain), "commit", "-q", "-m", "docs and src")
	gittest.Git(t, w.at(plain), "push", "-q", "origin", plain)

	fromRoot, err := w.usecase.Run(t.Context(), clean.Request{Worktrees: []string{"docs"}, Force: true}.From(domain.Caller{Cwd: w.at(plain)}), domain.RunOptions{})
	require.NoError(t, err)
	fromInside, err := w.usecase.Run(t.Context(), removing("src").From(domain.Caller{Cwd: filepath.Join(w.at(plain), "apps")}), domain.RunOptions{})
	require.NoError(t, err)

	assert.Equal(t, []string{"docs"}, fromRoot.Plan.Absent)
	assert.Equal(t, []string{"src"}, fromInside.Plan.Absent)
	assert.True(t, w.stands(t, plain))
}

func TestAWorktreeIsNamedByItsRootHoweverItIsSpelled(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.Symlink(w.at(plain), filepath.Join(w.root, "link")))

	for name, req := range map[string]clean.Request{
		"absolute": removing(w.at(plain)),
		"dot":      removing(".").From(domain.Caller{Cwd: w.at(plain)}),
		"symlink":  removing("link").From(domain.Caller{Cwd: w.root}),
	} {
		planned, _ := w.plan(t, req)
		require.Len(t, planned.Worktrees, 1, name)
		assert.Equal(t, plain, planned.Worktrees[0].Worktree, name)
	}
}

// On a disk that ignores case, as macOS's does, a root spelled in another case was not git's spelling: already absent, exit 0.
func TestAWorktreeIsNamedByItsRootInAnotherCase(t *testing.T) {
	w := newWorld(t, seed{})
	if _, err := os.Stat(strings.ToUpper(w.at(plain))); err != nil {
		t.Skip("this filesystem tells case apart")
	}

	planned, _ := w.plan(t, removing("WT-FEAT-PLAIN").From(domain.Caller{Cwd: w.root}))

	require.Len(t, planned.Worktrees, 1)
	assert.Equal(t, plain, planned.Worktrees[0].Worktree)
}

func TestAWorktreeOnNoBranchIsRemovedByItsPath(t *testing.T) {
	w := newWorld(t, seed{})
	path := w.at("detached")

	result, report := w.apply(t, removing(path))

	assert.Equal(t, map[string]domain.Status{path: domain.StatusDone}, statuses(result.Items))
	assert.NoDirExists(t, path)
	assert.Len(t, eventsOf[domain.WorktreeRemoved](report.All()), 1)
}

// status offers this fix for a gone worktree on no branch: its path is what is left to name it by.
func TestAWorktreeOnNoBranchWhoseDirectoryIsGoneIsRemovedByItsPath(t *testing.T) {
	w := newWorld(t, seed{})
	path := w.at("detached")
	require.NoError(t, os.RemoveAll(path))

	result, report := w.apply(t, removing(path))

	assert.Equal(t, map[string]domain.Status{path: domain.StatusDone}, statuses(result.Items))
	assert.NotContains(t, gittest.Output(t, w.main, "worktree", "list"), path)
	assert.Len(t, eventsOf[domain.WorktreeRemoved](report.All()), 1)
}

// The proof caught it: a path through a symbolic link, or from where the caller stands, is not git's spelling of the worktree, which read as already absent.
func TestAWorktreeOnNoBranchIsFoundByAPathFromWhereTheCallerStands(t *testing.T) {
	w := newWorld(t, seed{})
	req := removing("wt-detached").From(domain.Caller{Cwd: w.root})

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	assert.Empty(t, outcome.Plan.Absent)
	assert.Equal(t, map[string]domain.Status{w.at("detached"): domain.StatusDone}, statuses(outcome.Result.Items))
	assert.NoDirExists(t, w.at("detached"))
}

func TestJobsUpRefuseTheRemovalUntilForcedThenAreNamed(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Jobs = func(context.Context) ([]run.Job, error) {
		return []run.Job{{Name: "web", WorkDir: w.at(plain), Up: true}, {Name: "api", WorkDir: w.at(parent), Up: true}}, nil
	}

	_, err := w.usecase.Run(t.Context(), removing(plain), domain.RunOptions{DryRun: true})
	refused := failureOf(t, err)
	_, warnings := w.plan(t, clean.Request{Worktrees: []string{plain}, Force: true})

	require.Len(t, refused.Fields, 1)
	assert.Equal(t, "clean.jobs_running", refused.Fields[0].Code)
	require.NotNil(t, refused.Fields[0].FollowUp)
	assert.Equal(t, "run down", refused.Fields[0].FollowUp.Command)
	assert.Equal(t, []string{"clean.jobs_left"}, codes(warnings))
}

func TestJobsTheDaemonCannotTellAreWarnedOfAndRefuseNothing(t *testing.T) {
	w := newWorld(t, seed{JobErr: run.ErrUnknown})

	_, warnings := w.plan(t, removing(plain))

	assert.Equal(t, []string{"clean.jobs_unknown"}, codes(warnings))
}

func TestAnOpenPullRequestRefusesTheRemovalAndEveryBranchIsAsked(t *testing.T) {
	w := newWorld(t, seed{PRs: []worktree.PullRequest{{Branch: pushed, State: worktree.PullRequestOpen, URL: "https://github.com/acme/web/pull/7"}}})

	_, err := w.usecase.Run(t.Context(), removing(plain, pushed), domain.RunOptions{DryRun: true})

	refused := failureOf(t, err)
	require.Len(t, refused.Fields, 1)
	assert.Equal(t, "clean.open_pull_request", refused.Fields[0].Code)
	assert.Equal(t, "https://github.com/acme/web/pull/7", refused.Fields[0].Params["url"])
	assert.ElementsMatch(t, []string{plain, pushed}, *w.asked, "a branch squash-merged then deleted on origin has its pull request too")
}

// The most common clean, a pull request squash-merged on GitHub and its branch deleted there, was refused: the commits GitHub keeps as the pull request's head counted as unpushed.
func TestCommitsAMergedPullRequestHoldsAsItsHeadAreNotUnpushed(t *testing.T) {
	for name, c := range map[string]struct {
		later   bool
		prErr   error
		removed bool
	}{
		"merged_whole":      {removed: true},
		"committed_since":   {later: true},
		"forge_unavailable": {prErr: worktree.ErrPullRequestsUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, seed{})
			prs := []worktree.PullRequest{{Branch: local, State: worktree.PullRequestMerged, Head: gittest.Output(t, w.main, "rev-parse", local)}}
			w.usecase.PullRequests = func(context.Context, []string) ([]worktree.PullRequest, error) { return prs, c.prErr }
			if c.later {
				gittest.Git(t, w.at(local), "commit", "-q", "--allow-empty", "-m", "after the merge")
			}

			_, err := w.usecase.Run(t.Context(), removing(local), domain.RunOptions{DryRun: true})

			if c.removed {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, map[string][]string{"worktree[0]": {"clean.unpushed"}}, fieldCodes(failureOf(t, err).Fields))
		})
	}
}

func TestTheDataAWorktreeHoldsInSharedServicesIsNamedAsKept(t *testing.T) {
	w := newWorld(t, seed{})
	w.record(t, plain, worktree.Meta{SourceBranch: "main", Namespaces: []string{"acme_feat_plain"}})

	_, warnings := w.plan(t, removing(plain))

	require.Equal(t, []string{"clean.data_kept"}, codes(warnings))
	assert.Equal(t, "acme_feat_plain", warnings[0].Params["namespaces"])
}

// Only the records of worktrees were read: a branch no worktree holds lost its record, and the data it named went unmentioned.
func TestTheDataABranchNoWorktreeHoldsKeepsIsNamedToo(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "feat/left", "main")
	w.record(t, "feat/left", worktree.Meta{SourceBranch: "main", Namespaces: []string{"acme_feat_left"}})

	_, warnings := w.plan(t, removing("feat/left"))

	require.Equal(t, []string{"clean.data_kept"}, codes(warnings))
	assert.Equal(t, "acme_feat_left", warnings[0].Params["namespaces"])
}

func TestACancelSkipsTheWorktreesNotStarted(t *testing.T) {
	w := newWorld(t, seed{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, _ := w.applyIn(ctx, t, removing(plain, pushed))

	assert.Equal(t, map[string]domain.Status{plain: domain.StatusSkipped, pushed: domain.StatusSkipped}, statuses(result.Items))
	assert.True(t, w.stands(t, plain))
}

func TestARemovalUnderTheCallersFeetEndsOnTheMainWorktree(t *testing.T) {
	w := newWorld(t, seed{})
	req := removing(plain).From(domain.Caller{Cwd: filepath.Join(w.at(plain), "apps")})

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, outcome.Result.FollowUps, 1)
	assert.Equal(t, clean.CodeGo, outcome.Result.FollowUps[0].Code)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}}, outcome.Result.FollowUps[0].Answers)
}

// The first worktree git listed was taken for the main one: in a bare repository, the step out led to its entry, which has no working tree.
func TestARemovalUnderTheCallersFeetInABareRepositoryStepsNowhere(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Repository.Git = bare{w.repo}

	result, _ := w.apply(t, removing(plain).From(domain.Caller{Cwd: w.at(plain)}))

	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Empty(t, result.FollowUps)
}

// The caller was compared as given: one standing in the worktree through a symbolic link was not told to step out.
func TestARemovalUnderTheCallersFeetIsFoundThroughASymbolicLink(t *testing.T) {
	w := newWorld(t, seed{})
	link := filepath.Join(w.root, "link")
	require.NoError(t, os.Symlink(w.at(plain), link))

	result, _ := w.apply(t, removing(plain).From(domain.Caller{Cwd: link}))

	require.Len(t, result.FollowUps, 1)
	assert.Equal(t, clean.CodeGo, result.FollowUps[0].Code)
}
