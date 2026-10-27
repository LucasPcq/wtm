package teardown_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACommitNoOtherBranchHoldsMakesAWorktreeUnsafe(t *testing.T) {
	w := newWorld(seed{})

	facts := w.facts(t, base(), plain, stacked)

	assert.Empty(t, w.removal(t, facts, plain).Blockers)
	unsafe := w.removal(t, facts, stacked)
	assert.Equal(t, []worktree.Blocker{worktree.BlockerUnpushed}, unsafe.Blockers)
	assert.Equal(t, 1, unsafe.Unpushed)
	assert.True(t, unsafe.Unsafe())
}

func TestWorktreesRemovedTogetherHoldNothingForEachOther(t *testing.T) {
	w := newWorld(seed{})

	alone := w.facts(t, base(), local)
	together := w.facts(t, base(), local, stacked)

	assert.Empty(t, w.removal(t, alone, local).Blockers, "feat/stacked, which stays, holds its commit")
	assert.Equal(t, []worktree.Blocker{worktree.BlockerUnpushed}, w.removal(t, together, local).Blockers)
}

func TestSettlingNamesTheJobsUpAndTheOpenPullRequestAndAMergedHeadHoldsTheCommits(t *testing.T) {
	w := newWorld(seed{})
	facts := w.facts(t, base(), plain, local)
	worktrees := w.worktrees(t, facts, plain, local)
	prs := []worktree.PullRequest{
		{Branch: plain, State: worktree.PullRequestOpen, URL: "https://github.com/acme/web/pull/1"},
		{Branch: local, State: worktree.PullRequestMerged, Head: "local work"},
	}

	facts.Settle(worktrees, []run.Job{{Name: "web", WorkDir: at(plain), Up: true}}, prs)

	assert.Equal(t, []worktree.Blocker{worktree.BlockerOpenPullRequest, worktree.BlockerJobsRunning}, w.removal(t, facts, plain).Blockers)
	assert.Empty(t, w.removal(t, facts, local).Blockers, "its merged pull request's head is its tip")
}

func TestARefusalIsWordedUnderTheCommandAsking(t *testing.T) {
	w := newWorld(seed{})
	facts := w.facts(t, base(), stacked)
	facts.Settle(w.worktrees(t, facts, stacked), []run.Job{{Name: "web", WorkDir: at(stacked), Up: true}}, nil)
	forced := []domain.Answered{{Path: domain.PathWorktree, Values: []string{stacked}}, {Path: "force", Values: []string{"true"}}}

	refused := teardown.Refusal(teardown.RefusalParams{Command: command, Blocked: []teardown.Removal{w.removal(t, facts, stacked)}, Forced: forced})

	assert.Equal(t, domain.KindRefused, refused.Kind)
	assert.Equal(t, "sweep.unsafe", refused.Code)
	require.Len(t, refused.Fields, 2)
	assert.Equal(t, "sweep.unpushed", refused.Fields[0].Code)
	assert.Equal(t, "worktree[0]", refused.Fields[0].Path)
	assert.Equal(t, "sweep.jobs_running", refused.Fields[1].Code)
	require.NotNil(t, refused.Fields[1].FollowUp)
	assert.Equal(t, "sweep.stop_jobs", refused.Fields[1].FollowUp.Code)
	assert.Equal(t, &domain.FollowUp{Command: command, Answers: forced, Code: "sweep.force", Message: "remove anyway"}, refused.FollowUp)
}

func TestTheWarningsAboutARemovalStayOnlyForTheWorktreesThatWent(t *testing.T) {
	warnings := []domain.Warning{
		{Code: command + teardown.JobsLeft, Params: domain.Params{"worktree": plain}},
		{Code: command + teardown.DataKept, Params: domain.Params{"worktree": parent}},
		{Code: command + ".kept", Params: domain.Params{"worktree": parent}},
	}
	items := []domain.Unit[struct{}]{{Subject: plain, Detail: &struct{}{}}, {Subject: parent}}

	kept := teardown.OfRemoved(warnings, items)

	assert.Equal(t, []string{"sweep.jobs_left", "sweep.kept"}, codes(kept), "a warning not about a removal stays")
}
