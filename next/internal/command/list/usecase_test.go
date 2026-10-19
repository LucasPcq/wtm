package list_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachWorktreeIsReadTheMainOneFirstThenTheOldest(t *testing.T) {
	older, newer := created.Format(time.RFC3339), created.Add(time.Hour).Format(time.RFC3339)
	w := newWorld(seed{
		Git: gittest.FakeParams{
			Main: mainPath, Branch: "main", Remote: []string{"main", "feat/a"},
			Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/b"), linked("feat/c")},
			Dirty:     []string{"/repo/.trees/feat/a"},
			Commits: map[worktree.Ref][]string{
				worktree.LocalBranch{Name: "feat/a"}: {"one", "two"},
			},
		},
		Meta: map[string]worktree.Meta{"feat/a": {CreatedAt: newer}, "feat/b": {CreatedAt: older}},
		Jobs: []run.Job{{Name: "web", WorkDir: "/repo/.trees/feat/a", Up: true}, {Name: "api", WorkDir: "/repo/.trees/feat/a"}},
	})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{"main", "feat/b", "feat/a", "feat/c"}, subjects(read))
	assert.Empty(t, read.Warnings)
	a := detailOf(t, read, "feat/a")
	assert.Equal(t, list.Detail{
		Branch: "feat/a", Path: "/repo/.trees/feat/a", Dirty: true, CommitsAhead: 2, CreatedAt: a.CreatedAt,
		Origin: &list.Origin{Ahead: 2, State: worktree.Ahead}, Services: []string{"web"}, Actions: a.Actions,
	}, a)
	assert.Equal(t, created.Add(time.Hour), *a.CreatedAt)
	assert.True(t, detailOf(t, read, "main").Main)
	assert.Equal(t, &list.Origin{State: worktree.UpToDate}, detailOf(t, read, "main").Origin)
	assert.Nil(t, detailOf(t, read, "feat/c").CreatedAt, "a worktree without a meta record has no creation date")
	assert.Nil(t, detailOf(t, read, "feat/c").Origin)
}

func TestAReadChangesNothingAndTakesNoLock(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, Meta: map[string]worktree.Meta{"feat/a": {CreatedAt: created.Format(time.RFC3339)}}})

	_, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	for _, call := range append(w.git.Calls(), w.meta.Calls()...) {
		assert.Regexp(t, `^(Worktrees|Branches|AheadBehind|Locate|Dirty|Count|Load) `, call)
	}
}

func TestALockedARebasingAndADetachedWorktreeAreSaidSo(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{
		{Path: "/repo/.trees/locked", State: worktree.OnBranch{Branch: "locked"}, Locked: true},
		{Path: "/repo/.trees/rebasing", State: worktree.Rebasing{Branch: "rebasing"}},
		{Path: "/repo/.trees/detached", State: worktree.Detached{}},
	}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, detailOf(t, read, "locked").Locked)
	assert.True(t, detailOf(t, read, "rebasing").Rebasing)
	detached := detailOf(t, read, "/repo/.trees/detached")
	assert.Empty(t, detached.Branch, "a worktree on no branch is named by its path")
	assert.Zero(t, detached.CommitsAhead)
}

func TestAWorktreeWhoseDirectoryIsGoneIsMissingNotClean(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/gone")}, Gone: []string{"/repo/.trees/feat/gone"}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	gone := detailOf(t, read, "feat/gone")
	assert.True(t, gone.Missing)
	assert.False(t, gone.Dirty)
	assert.Nil(t, read.Failure())
}

func TestAWorktreeThatCannotBeReadFailsAlone(t *testing.T) {
	broken := errors.New("git status failed")
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Dirty": broken}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, read.Items, 2)
	failed := read.Items[1]
	if read.Items[0].Status == domain.StatusFailed {
		failed = read.Items[0]
	}
	assert.Equal(t, domain.StatusFailed, failed.Status)
	assert.Equal(t, list.CodeUnreadable, failed.Err.Code)
	assert.ErrorIs(t, failed.Err, broken)
	assert.Equal(t, failed.Err, read.Failure())
}

func TestGitHubIsAskedOnlyWhenTheRequestAsks(t *testing.T) {
	pr := worktree.PullRequest{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/a"}
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, PRs: worktree.PullRequests{Open: []worktree.PullRequest{pr}}})

	without, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	with, err := w.usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, *w.asked)
	assert.Nil(t, detailOf(t, without, "feat/a").PR)
	assert.Equal(t, &list.PR{Number: 12, URL: pr.URL}, detailOf(t, with, "feat/a").PR)
	assert.Nil(t, detailOf(t, with, "main").PR)
}

func TestWhatCannotBeAskedIsAWarningNotAFailure(t *testing.T) {
	cases := map[string]struct {
		seed    seed
		codes   []string
		message string
	}{
		"prs_unavailable": {seed: seed{PRsErr: fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable)}, codes: []string{list.CodePRsUnavailable}, message: "pull requests unavailable: run `gh auth login`"},
		"prs_capped":      {seed: seed{PRs: worktree.PullRequests{Capped: true}}, codes: []string{list.CodePRsCapped}, message: "only the most recent open pull requests were listed: an older one is not shown"},
		"daemon_unknown":  {seed: seed{JobsErr: run.ErrUnknown}, codes: []string{list.CodeJobsUnknown}, message: "the run daemon is not running: which jobs are up is unknown"},
		"base_missing":    {seed: seed{Base: "develop"}, codes: []string{list.CodeBaseMissing}, message: "the base branch develop does not exist: commits ahead of it are not counted"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			read, err := newWorld(c.seed).usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})

			require.NoError(t, err)
			assert.Equal(t, c.codes, codes(read))
			assert.Equal(t, c.message, read.Warnings[0].Message)
			assert.Nil(t, read.Failure())
		})
	}
}

func TestJobsThatCannotBeToldAreNoServicesAtAll(t *testing.T) {
	unknown, err := newWorld(seed{JobsErr: run.ErrUnknown}).usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	none, err := newWorld(seed{}).usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)

	assert.Nil(t, detailOf(t, unknown, "main").Services)
	assert.Equal(t, []string{}, detailOf(t, none, "main").Services)
}

func TestARefreshReadsOriginFetchedFirst(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Remote: []string{"main"}, Worktrees: []worktree.Worktree{linked("feat/a")}, Origin: map[string][]string{"main": {"upstream"}}}})

	read, err := w.usecase.Run(t.Context(), list.Request{Fetch: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, &list.Origin{Behind: 1, State: worktree.Behind}, detailOf(t, read, "main").Origin)
}

func TestARepositoryWithoutConfigIsRefusedWithListsCode(t *testing.T) {
	_, err := newWorld(seed{ConfigErr: project.ErrConfigNotFound}).usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, list.CodeConfigNotFound, refused.Code)
}

func TestEachWorktreeReportsItsActionsAsCommandsWithTheirAnswers(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	actions := detailOf(t, read, "feat/a").Actions
	require.Len(t, actions, 5)
	assert.Equal(t, domain.FollowUp{
		Command: "run up",
		Answers: []domain.Answered{{Path: list.PathWorktree, Values: []string{"feat/a"}}},
		Argv:    []string{"wtm", "run", "up", "feat/a"},
		Code:    "list.run_up",
		Message: "Start the profile",
	}, actions[1])
}

func TestAPullRequestIsALinkToOpenNotACommand(t *testing.T) {
	followUp := list.FollowUpOf(list.ActionOpenPR, list.Detail{Branch: "feat/a", PR: &list.PR{Number: 12, URL: "https://github.com/acme/acme/pull/12"}})

	assert.Equal(t, domain.FollowUp{URL: "https://github.com/acme/acme/pull/12", Code: "list.open_pr", Message: "Open the pull request", Params: domain.Params{list.ParamNumber: "12"}}, followUp)
}

func TestAWorktreeOnNoBranchIsAnsweredByItsPath(t *testing.T) {
	followUp := list.FollowUpOf(list.ActionGo, list.Detail{Path: "/repo/.trees/x"})

	assert.Equal(t, "go", followUp.Command)
	assert.Equal(t, []domain.Answered{{Path: list.PathWorktree, Values: []string{"/repo/.trees/x"}}}, followUp.Answers)
}

func TestALockedWorktreeWhoseDirectoryIsGoneIsMissing(t *testing.T) {
	locked := linked("feat/locked")
	locked.Locked = true
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{locked}, Gone: []string{locked.Path}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, read.Failure())
	gone := detailOf(t, read, "feat/locked")
	assert.True(t, gone.Missing)
	assert.True(t, gone.Locked)
	assert.NotContains(t, w.git.Calls(), "Dirty "+locked.Path)
}

func TestTheEntryOfABareRepositoryIsNotAWorktree(t *testing.T) {
	git := gittest.FakeParams{Main: "/repo/bare.git", Branch: "main", Worktrees: []worktree.Worktree{linked("feat/a")}}
	w := newWorld(seed{Git: git})
	bare, err := w.git.Worktrees(t.Context())
	require.NoError(t, err)
	bare[0].State = worktree.Bare{}
	w.usecase.Repository.Git = snapshotOf{git: w.git, worktrees: bare}

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, read.Failure())
	assert.Equal(t, []string{"feat/a"}, subjects(read))
}

func TestABranchWithNothingToCountIsNotCountedNotUnreadable(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/orphan")}, Dirty: []string{"/repo/.trees/feat/orphan"}, Fail: map[string]error{"Count": errors.New("bad revision")}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, read.Failure())
	orphan := detailOf(t, read, "feat/orphan")
	assert.Zero(t, orphan.CommitsAhead)
	assert.True(t, orphan.Dirty, "what was read before the count is kept")
}

func TestAReadCancelledReportsItsWorktreesInterrupted(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Dirty": context.Canceled}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	for _, item := range read.Items {
		assert.Equal(t, domain.StatusCancelled, item.Status, item.Subject)
		assert.Equal(t, domain.ReasonInterrupted, item.Reason, item.Subject)
	}
	assert.Equal(t, domain.KindCancelled, read.Failure().Kind)
}

func TestARefreshFetchesOriginOnce(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Remote: []string{"main"}, Worktrees: []worktree.Worktree{linked("feat/a")}, Origin: map[string][]string{"main": {"upstream"}, "feat/a": {"pushed"}}}})

	_, err := w.usecase.Run(t.Context(), list.Request{Fetch: true}, domain.RunOptions{})

	require.NoError(t, err)
	fetches := 0
	for _, call := range w.git.Calls() {
		if strings.HasPrefix(call, "Fetch") {
			fetches++
			assert.Equal(t, "Fetch ", call)
		}
	}
	assert.Equal(t, 1, fetches)
}
