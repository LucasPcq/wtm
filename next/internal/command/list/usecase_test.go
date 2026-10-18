package list_test

import (
	"errors"
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
		Origin: &list.Origin{Ahead: 2, State: worktree.Ahead}, Services: []string{"web"},
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
		assert.Regexp(t, `^(Worktrees|Branches|AheadBehind|Dirty|Count|Load) `, call)
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
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Count": broken}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, read.Items, 2)
	assert.Equal(t, domain.StatusDone, read.Items[0].Status)
	failed := read.Items[1]
	assert.Equal(t, domain.StatusFailed, failed.Status)
	assert.Equal(t, list.CodeUnreadable, failed.Err.Code)
	assert.ErrorIs(t, failed.Err, broken)
	assert.Equal(t, failed.Err, read.Failure())
}

func TestGitHubIsAskedOnlyWhenTheRequestAsks(t *testing.T) {
	pr := worktree.PullRequest{Number: 12, URL: "https://github.com/acme/acme/pull/12", State: "open", Branch: "feat/a"}
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, PRs: []worktree.PullRequest{pr}})

	without, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	with, err := w.usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})
	require.NoError(t, err)

	assert.Equal(t, 1, *w.asked)
	assert.Nil(t, detailOf(t, without, "feat/a").PR)
	assert.Equal(t, &list.PR{Number: 12, URL: pr.URL, State: "open"}, detailOf(t, with, "feat/a").PR)
	assert.Nil(t, detailOf(t, with, "main").PR)
}

func TestWhatCannotBeAskedIsAWarningNotAFailure(t *testing.T) {
	cases := map[string]struct {
		seed    seed
		codes   []string
		message string
	}{
		"gh_signed_out":  {seed: seed{PRsErr: worktree.ErrGitHubSignedOut}, codes: []string{list.CodePRsUnavailable}, message: "GitHub not connected: run `gh auth login` to see pull requests"},
		"gh_missing":     {seed: seed{PRsErr: worktree.ErrGitHubMissing}, codes: []string{list.CodePRsUnavailable}, message: "GitHub CLI not installed: install gh to see pull requests"},
		"daemon_unknown": {seed: seed{JobsErr: run.ErrUnknown}, codes: []string{list.CodeJobsUnknown}, message: "the run daemon is not running: which jobs are up is unknown"},
		"base_missing":   {seed: seed{Base: "develop"}, codes: []string{list.CodeBaseMissing}, message: "the base branch develop does not exist: commits ahead of it are not counted"},
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

func TestARefreshFetchesTheBranchesOriginHasFirst(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Remote: []string{"main"}, Worktrees: []worktree.Worktree{linked("feat/a")}, Origin: map[string][]string{"main": {"upstream"}}}})

	read, err := w.usecase.Run(t.Context(), list.Request{Fetch: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Contains(t, w.git.Calls(), "FetchRef main")
	assert.NotContains(t, w.git.Calls(), "FetchRef feat/a")
	assert.Equal(t, &list.Origin{Behind: 1, State: worktree.Behind}, detailOf(t, read, "main").Origin)
}

func TestARepositoryWithoutConfigIsRefusedWithListsCode(t *testing.T) {
	_, err := newWorld(seed{ConfigErr: project.ErrConfigNotFound}).usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, list.CodeConfigNotFound, refused.Code)
}

func TestAFollowUpIsTheCommandLineOfTheActionWithItsLabel(t *testing.T) {
	followUp := newWorld(seed{}).usecase.FollowUp(list.Detail{Branch: "feat/a"}, list.ActionRunUp)

	assert.Equal(t, domain.FollowUp{Argv: []string{"wtm", "run_up", "feat/a"}, Code: "list.run_up", Message: "Start the profile"}, followUp)
}
