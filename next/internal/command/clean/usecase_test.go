package clean_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

func TestAPathInsideAWorktreeNamesIt(t *testing.T) {
	w := newWorld(t, seed{})
	req := removing("wt-feat-plain/apps").From(domain.Caller{Cwd: w.root})
	writeFile(t, filepath.Join(w.at(plain), "apps", "keep.txt"), "x")

	_, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true})

	assert.Equal(t, map[string][]string{"worktree[0]": {"clean.dirty"}}, fieldCodes(failureOf(t, err).Fields))
}

func TestAWorktreeOnNoBranchIsRemovedByItsPath(t *testing.T) {
	w := newWorld(t, seed{})
	path := w.at("detached")

	result, report := w.apply(t, removing(path))

	assert.Equal(t, map[string]domain.Status{path: domain.StatusDone}, statuses(result.Items))
	assert.NoDirExists(t, path)
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

func TestAnOpenPullRequestRefusesTheRemovalAndOnlyPushedBranchesAreAsked(t *testing.T) {
	w := newWorld(t, seed{PRs: []worktree.PullRequest{{Branch: pushed, State: worktree.PullRequestOpen, URL: "https://github.com/acme/web/pull/7"}}})

	_, err := w.usecase.Run(t.Context(), removing(plain, pushed), domain.RunOptions{DryRun: true})

	refused := failureOf(t, err)
	require.Len(t, refused.Fields, 1)
	assert.Equal(t, "clean.open_pull_request", refused.Fields[0].Code)
	assert.Equal(t, "https://github.com/acme/web/pull/7", refused.Fields[0].Params["url"])
	assert.Equal(t, []string{pushed}, *w.asked)
}

func TestTheDataAWorktreeHoldsInSharedServicesIsNamedAsKept(t *testing.T) {
	w := newWorld(t, seed{})
	w.record(t, plain, worktree.Meta{SourceBranch: "main", Namespaces: []string{"acme_feat_plain"}})

	_, warnings := w.plan(t, removing(plain))

	require.Equal(t, []string{"clean.data_kept"}, codes(warnings))
	assert.Equal(t, "acme_feat_plain", warnings[0].Params["namespaces"])
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
