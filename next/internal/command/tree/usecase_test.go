package tree_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheTreeListsParentsBeforeTheirChildrenRootsMainFirst(t *testing.T) {
	w := newWorld(seed{
		Git: gittest.FakeParams{Worktrees: []worktree.Worktree{
			linked("feat/a"), linked("feat/a1"), linked("feat/b"), linked("loose"), linked("y"), linked("z"), linked("feat/x"),
			{Path: "/repo/.trees/detached", State: worktree.Detached{}},
		}},
		Meta: map[string]worktree.Meta{
			"feat/b":  from("main", "2026-10-02T00:00:00Z"),
			"feat/a":  from("main", "2026-10-01T00:00:00Z"),
			"feat/a1": from("feat/a", "2026-10-03T00:00:00Z"),
			"y":       from("z", "2026-10-04T00:00:00Z"),
			"z":       from("y", "2026-10-05T00:00:00Z"),
			"feat/x":  from("gone", "2026-10-06T00:00:00Z"),
		},
	})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{"main", "feat/a", "feat/a1", "feat/b", "loose", "y", "z", "gone", "feat/x"}, subjects(read))
	assert.Nil(t, read.Failure())
	main := detailOf(t, read, "main")
	assert.True(t, main.Main)
	assert.Nil(t, main.Parent)
	assert.Equal(t, []string{"feat/a", "feat/b"}, main.Children)
	a1 := detailOf(t, read, "feat/a1")
	assert.Equal(t, "feat/a", *a1.Parent)
	assert.Equal(t, "/repo/.trees/feat/a1", *a1.Path)
	gone := detailOf(t, read, "gone")
	assert.Equal(t, tree.Detail{Branch: "gone", Virtual: true, Children: []string{"feat/x"}, Actions: []domain.FollowUp{}}, gone)
	assert.True(t, detailOf(t, read, "y").Cycle)
	assert.Nil(t, detailOf(t, read, "y").Parent, "a member of a cycle is drawn as a root")
	assert.Nil(t, detailOf(t, read, "y").Behind, "a member of a cycle has no parent to sync onto")
	assert.False(t, detailOf(t, read, "feat/a").Cycle)
}

func TestADetachedWorktreeIsNoNodeOfTheTree(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{
		{Path: "/repo/.trees/one", State: worktree.Detached{}},
		{Path: "/repo/.trees/two", State: worktree.Detached{}},
	}}})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, subjects(read))
}

func TestAheadAndBehindAreCountedAgainstTheParent(t *testing.T) {
	w := newWorld(seed{
		Git: gittest.FakeParams{
			Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/a1"), linked("feat/b"), linked("loose"), linked("feat/x"), linked("feat/o")},
			Commits: commits(map[string][]string{
				"main":    {"m1"},
				"feat/a":  {"m1", "a1", "a2"},
				"feat/a1": {"m1", "a1", "a2", "c1"},
				"feat/b":  {"b1"},
				"loose":   {"m1", "l1"},
			}),
		},
		Meta: map[string]worktree.Meta{
			"feat/a":  from("main", ""),
			"feat/a1": from("feat/a", ""),
			"feat/b":  from("main", ""),
			"feat/x":  from("gone", ""),
			"feat/o":  from("origin/feat/o", ""),
		},
	})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	cases := map[string]struct {
		ahead, behind *int
		needsSync     bool
	}{
		"main":    {},
		"feat/a":  {ahead: count(2), behind: count(0)},
		"feat/a1": {ahead: count(1), behind: count(0)},
		"feat/b":  {ahead: count(1), behind: count(1), needsSync: true},
		"loose":   {ahead: count(1), behind: count(0)},
		"feat/x":  {},
		"feat/o":  {},
	}
	for branch, c := range cases {
		t.Run(worktree.PathSegment(branch), func(t *testing.T) {
			detail := detailOf(t, read, branch)

			assert.Equal(t, c.ahead, detail.Ahead)
			assert.Equal(t, c.behind, detail.Behind)
			assert.Equal(t, c.needsSync, detail.NeedsSync)
		})
	}
}

func TestARootIsNotCountedWhenTheBaseBranchIsMissing(t *testing.T) {
	w := newWorld(seed{Base: "develop", Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("loose"), linked("feat/a")}}, Meta: map[string]worktree.Meta{"feat/a": from("main", "")}})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, detailOf(t, read, "loose").Ahead)
	assert.NotNil(t, detailOf(t, read, "feat/a").Ahead, "a child is counted against its parent")
	assert.Equal(t, []string{tree.CodeBaseMissing}, codes(read))
	assert.Equal(t, "the base branch develop does not exist: the roots are not counted against it", read.Warnings[0].Message)
}

func TestAReadChangesNothingAndReadsEachWorktreeOnce(t *testing.T) {
	w := newWorld(seed{
		Git:  gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/b")}},
		Meta: map[string]worktree.Meta{"feat/a": from("main", ""), "feat/b": from("feat/a", "")},
	})

	_, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	seen := map[string]int{}
	for _, call := range append(w.git.Calls(), w.meta.Calls()...) {
		assert.Regexp(t, `^(Worktrees|Branches|AheadBehind|Locate|Dirty|Count|Load) `, call)
		seen[call]++
	}
	assert.Equal(t, 1, seen["Worktrees "])
	assert.Equal(t, 1, seen["Locate /repo/.trees/feat/a"])
	assert.Equal(t, 1, seen["Dirty /repo/.trees/feat/a"])
	assert.Equal(t, 1, seen["Load feat/a"])
}

func TestALockedARebasingAndADirtyWorktreeAreSaidSo(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{
		Worktrees: []worktree.Worktree{
			{Path: "/repo/.trees/locked", State: worktree.OnBranch{Branch: "locked"}, Locked: true},
			{Path: "/repo/.trees/rebasing", State: worktree.Rebasing{Branch: "rebasing"}},
			linked("dirty"),
		},
		Dirty: []string{"/repo/.trees/dirty"},
	}})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, detailOf(t, read, "locked").Locked)
	assert.True(t, detailOf(t, read, "rebasing").Rebasing)
	assert.True(t, detailOf(t, read, "dirty").Dirty)
}

func TestAWorktreeWhoseDirectoryIsGoneIsMissingNotClean(t *testing.T) {
	locked := linked("feat/locked")
	locked.Locked = true
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/gone"), locked}, Gone: []string{"/repo/.trees/feat/gone", locked.Path}}})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, read.Failure())
	for _, branch := range []string{"feat/gone", "feat/locked"} {
		gone := detailOf(t, read, branch)
		assert.True(t, gone.Missing, branch)
		assert.False(t, gone.Dirty, branch)
		assert.Nil(t, gone.Ahead, branch)
	}
	assert.NotContains(t, w.git.Calls(), "Dirty "+locked.Path)
}

func TestAWorktreeThatCannotBeReadFailsAloneKeepingItsPlace(t *testing.T) {
	broken := errors.New("git status failed")
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Dirty": broken}}, Meta: map[string]worktree.Meta{"feat/a": from("main", "")}})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, read.Items, 2)
	failed := read.Items[1]
	assert.Equal(t, domain.StatusFailed, failed.Status)
	assert.Equal(t, tree.CodeUnreadable, failed.Err.Code)
	assert.ErrorIs(t, failed.Err, broken)
	assert.Equal(t, "main", *failed.Detail.Parent)
	assert.Equal(t, read.Items[0].Err, read.Failure(), "the first node not read")
}

func TestAReadCancelledReportsItsWorktreesInterrupted(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Dirty": context.Canceled}}})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	for _, item := range read.Items {
		assert.Equal(t, domain.StatusCancelled, item.Status, item.Subject)
	}
	assert.Equal(t, domain.KindCancelled, read.Failure().Kind)
}

func TestGitHubIsAskedOnlyWhenTheRequestAsksOfTheBranchesOfTheTree(t *testing.T) {
	merged := worktree.PullRequest{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/a", State: worktree.PullRequestMerged}
	w := newWorld(seed{
		Git:  gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), {Path: "/repo/.trees/detached", State: worktree.Detached{}}}},
		Meta: map[string]worktree.Meta{"feat/a": from("gone", "")},
		PRs:  []worktree.PullRequest{merged},
	})

	without, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	with, err := w.usecase.Run(t.Context(), tree.Request{WithPRs: true}, domain.RunOptions{})
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"main", "feat/a"}}, *w.asked)
	assert.Nil(t, detailOf(t, without, "feat/a").PR)
	assert.Equal(t, &tree.PR{Number: 12, URL: merged.URL, State: worktree.PullRequestMerged}, detailOf(t, with, "feat/a").PR)
	assert.Nil(t, detailOf(t, with, "main").PR)
}

func TestWhatCannotBeAskedIsAWarningNotAFailure(t *testing.T) {
	cases := map[string]struct {
		seed    seed
		code    string
		message string
	}{
		"prs_unavailable": {seed: seed{PRsErr: fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable)}, code: tree.CodePRsUnavailable, message: "pull requests unavailable: run `gh auth login`"},
		"daemon_unknown":  {seed: seed{JobsErr: run.ErrUnknown}, code: tree.CodeJobsUnknown, message: "the run daemon is not running: which jobs are up is unknown"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			read, err := newWorld(c.seed).usecase.Run(t.Context(), tree.Request{WithPRs: true}, domain.RunOptions{})

			require.NoError(t, err)
			assert.Equal(t, []string{c.code}, codes(read))
			assert.Equal(t, c.message, read.Warnings[0].Message)
			assert.Nil(t, read.Failure())
		})
	}
}

func TestThePullRequestsFoundBeforeGitHubFailedAreKept(t *testing.T) {
	found := worktree.PullRequest{Number: 3, URL: "u", Branch: "main", State: worktree.PullRequestOpen}
	read, err := newWorld(seed{PRs: []worktree.PullRequest{found}, PRsErr: fmt.Errorf("%w: gh api graphql: boom", worktree.ErrPullRequestsUnavailable)}).usecase.Run(t.Context(), tree.Request{WithPRs: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{tree.CodePRsUnavailable}, codes(read))
	assert.Equal(t, &tree.PR{Number: 3, URL: "u", State: worktree.PullRequestOpen}, detailOf(t, read, "main").PR)
}

func TestTheJobsUpInAWorktreeAreItsServices(t *testing.T) {
	w := newWorld(seed{
		Git:  gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}},
		Jobs: []run.Job{{Name: "web", WorkDir: "/repo/.trees/feat/a", Up: true}, {Name: "api", WorkDir: "/repo/.trees/feat/a"}},
	})
	unknown, err := newWorld(seed{JobsErr: run.ErrUnknown}).usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})
	require.NoError(t, err)

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{"web"}, detailOf(t, read, "feat/a").Services)
	assert.Equal(t, []string{}, detailOf(t, read, "main").Services)
	assert.Nil(t, detailOf(t, unknown, "main").Services)
}

func TestWhereABranchStandsAgainstOriginIsSaid(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Remote: []string{"main"}, Origin: map[string][]string{"main": {"upstream"}}}})

	read, err := w.usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, &worktree.Origin{State: worktree.UpToDate}, detailOf(t, read, "main").Origin, "an origin not fetched is not counted")
}

func TestARepositoryWithoutConfigIsRefusedWithTreesCode(t *testing.T) {
	_, err := newWorld(seed{ConfigErr: project.ErrConfigNotFound}).usecase.Run(t.Context(), tree.Request{}, domain.RunOptions{})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, tree.CodeConfigNotFound, refused.Code)
}
