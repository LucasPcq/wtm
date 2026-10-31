package list_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

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
			Changes:   map[string][]worktree.Change{"/repo/.trees/feat/a": {{Path: "a.go", Status: worktree.ChangeModified}, {Path: "b.go", Status: worktree.ChangeUntracked}}},
			Commits: map[worktree.Ref][]string{
				worktree.LocalBranch{Name: "feat/a"}: {"one", "two"},
			},
		},
		Meta: map[string]worktree.Meta{"feat/a": {CreatedAt: newer}, "feat/b": {CreatedAt: older}},
		Jobs: []run.Job{
			{Name: "web", WorkDir: "/repo/.trees/feat/a", Profile: "dev", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, URL: "http://web.feat-a.acme.localhost", StartedAt: created},
			{Name: "api", WorkDir: "/repo/.trees/feat/a", Profile: "dev", Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn, URL: "http://api.feat-a.acme.localhost", ExitCode: ptr(1)},
		},
	})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{"main", "feat/b", "feat/a", "feat/c"}, subjects(read))
	assert.Empty(t, read.Warnings)
	a := detailOf(t, read, "feat/a")
	assert.Equal(t, list.Detail{
		Branch: "feat/a", Path: "/repo/.trees/feat/a", Record: worktree.RecordRead, Dirty: true, CommitsAhead: 2, Ahead: ptr(2), Changes: &list.Changes{Changed: 1, Untracked: 1},
		CreatedAt: a.CreatedAt, LastCommitAt: &gittest.Committed, Origin: &worktree.Origin{Ahead: 2, State: worktree.Ahead},
		Condition: worktree.ConditionChanged,
		Services:  []string{"web"}, URLs: []string{"http://web.feat-a.acme.localhost"}, Actions: a.Actions,
		Jobs: []list.Job{
			{Name: "api", Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn, Profile: "dev", URL: "http://api.feat-a.acme.localhost", ExitCode: ptr(1)},
			{Name: "web", Kind: project.Service, State: run.Running, Hold: run.HoldOwn, Profile: "dev", URL: "http://web.feat-a.acme.localhost", StartedAt: &created},
		},
	}, a)
	assert.Equal(t, created.Add(time.Hour), *a.CreatedAt)
	assert.NotNil(t, offerOf(t, a.Actions, worktree.ActionReparent).FollowUp, "wtm recorded feat/a: it can change its parent")
	refused := offerOf(t, detailOf(t, read, "feat/c").Actions, worktree.ActionReparent).Refused
	require.NotNil(t, refused)
	assert.Equal(t, worktree.UnofferedUnrecorded, *refused)
	assert.True(t, detailOf(t, read, "main").Main)
	assert.Equal(t, &worktree.Origin{State: worktree.UpToDate}, detailOf(t, read, "main").Origin)
	assert.Nil(t, detailOf(t, read, "feat/c").CreatedAt, "a worktree without a meta record has no creation date")
	assert.Nil(t, detailOf(t, read, "feat/c").Origin)
}

func TestAReadChangesNothingAndTakesNoLock(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, Meta: map[string]worktree.Meta{"feat/a": {CreatedAt: created.Format(time.RFC3339)}}})

	_, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	for _, call := range append(w.git.Calls(), w.meta.Calls()...) {
		assert.Regexp(t, `^(Worktrees|Branches|CommitDates|AheadBehind|Locate|Dirty|AheadBehindOf|Load) `, call)
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
	assert.Equal(t, worktree.ConditionRebasing, detailOf(t, read, "rebasing").Condition)
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
	pr := worktree.PullRequest{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/a", State: worktree.PullRequestMerged}
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), {Path: "/repo/.trees/detached", State: worktree.Detached{}}}}, PRs: []worktree.PullRequest{pr}})

	without, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	with, err := w.usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"main", "", "feat/a"}}, *w.asked, "the live branches as they are: the adapter skips the empty one")
	assert.Nil(t, detailOf(t, without, "feat/a").PR)
	assert.Equal(t, &list.PR{Number: 12, URL: pr.URL, State: worktree.PullRequestMerged}, detailOf(t, with, "feat/a").PR, "the newest pull request, whatever its state")
	assert.Nil(t, detailOf(t, with, "main").PR)
}

func TestWhatCannotBeAskedIsAWarningNotAFailure(t *testing.T) {
	cases := map[string]struct {
		seed    seed
		codes   []string
		message string
	}{
		"prs_unavailable": {seed: seed{PRsErr: fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable)}, codes: []string{list.CodePRsUnavailable}, message: "pull requests unavailable: run `gh auth login`"},
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

func TestAnInterruptedReadWarnsOfNothingItCouldNotAsk(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	w := newWorld(seed{JobsErr: run.ErrUnknown, PRsErr: context.Canceled, Git: gittest.FakeParams{Fail: map[string]error{"Fetch": context.Canceled}}})

	read, err := w.usecase.Run(ctx, list.Request{WithPRs: true, Fetch: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Empty(t, read.Warnings)
	assert.Equal(t, domain.KindCancelled, read.Failure().Kind, "every worktree read, the read still ends interrupted")
}

func TestARecordThatDoesNotReadKeepsItsWorktreeAndIsAWarning(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, Unreadable: []string{"feat/a"}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, read.Failure())
	item := read.Items[slices.IndexFunc(read.Items, func(item list.Item) bool { return item.Subject == "feat/a" })]
	assert.Equal(t, domain.StatusDone, item.Status)
	require.Len(t, item.Warnings, 1)
	assert.Equal(t, list.CodeMetaUnreadable, item.Warnings[0].Code)
	assert.Nil(t, item.Detail.CreatedAt)
	refused := offerOf(t, item.Detail.Actions, worktree.ActionReparent).Refused
	require.NotNil(t, refused, "a record that does not read refuses reparent with why")
	assert.Equal(t, worktree.UnofferedUnreadable, *refused)
}

func TestARecordANewerWtmWroteIsReadAsUnreadableAndSaysSo(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, Meta: map[string]worktree.Meta{"feat/a": {Schema: 99, CreatedAt: "2026-10-28T12:00:00Z"}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	item := read.Items[slices.IndexFunc(read.Items, func(item list.Item) bool { return item.Subject == "feat/a" })]
	assert.Equal(t, domain.StatusDone, item.Status)
	require.Len(t, item.Warnings, 1)
	assert.Equal(t, list.CodeMetaUnreadable, item.Warnings[0].Code)
	assert.Contains(t, item.Warnings[0].Message, "is at schema 99, newer than the 1 this wtm writes")
	assert.Nil(t, item.Detail.CreatedAt, "nothing is shown from a record whose meaning this wtm does not know")
}

func TestARecordThatDoesNotReadIsAWarningOnAWorktreeNotReadToo(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Dirty": errors.New("broken index")}}, Unreadable: []string{"feat/a"}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	item := read.Items[slices.IndexFunc(read.Items, func(item list.Item) bool { return item.Subject == "feat/a" })]
	assert.Equal(t, domain.StatusFailed, item.Status)
	require.Len(t, item.Warnings, 1)
	assert.Equal(t, list.CodeMetaUnreadable, item.Warnings[0].Code)
}

func TestARefreshThatCannotFetchIsAWarning(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Fail: map[string]error{"Fetch": errors.New("could not read from remote repository")}}})

	read, err := w.usecase.Run(t.Context(), list.Request{Fetch: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{list.CodeFetchFailed}, codes(read))
	assert.Contains(t, read.Warnings[0].Message, "could not read from remote repository")
}

func TestDatesThatCannotBeReadAreAWarningNotAFailure(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Fail: map[string]error{"CommitDates": errors.New("for-each-ref failed")}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{list.CodeDatesUnreadable}, codes(read))
	assert.Nil(t, detailOf(t, read, "main").LastCommitAt)
}

func TestJobsThatCannotBeToldAreNoServicesAtAll(t *testing.T) {
	unknown, err := newWorld(seed{JobsErr: run.ErrUnknown}).usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	none, err := newWorld(seed{}).usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)

	assert.Nil(t, detailOf(t, unknown, "main").Services)
	assert.Nil(t, detailOf(t, unknown, "main").Jobs)
	assert.Equal(t, []string{}, detailOf(t, none, "main").Services)
	assert.Equal(t, []list.Job{}, detailOf(t, none, "main").Jobs)
}

func TestARefreshReadsOriginFetchedFirst(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Remote: []string{"main"}, Worktrees: []worktree.Worktree{linked("feat/a")}, Origin: map[string][]string{"main": {"upstream"}}}})

	read, err := w.usecase.Run(t.Context(), list.Request{Fetch: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, &worktree.Origin{Behind: 1, State: worktree.Behind}, detailOf(t, read, "main").Origin)
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
	require.Len(t, actions, len(worktree.Actions()))
	assert.Equal(t, worktree.ActionRunUp, actions[3].Action)
	assert.Equal(t, &domain.FollowUp{
		Command: domain.CommandRunUp,
		Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}},
		Code:    "list.run_up",
		Message: "Start the profile",
	}, actions[3].FollowUp)
}

func TestAnActionAWorktreeDoesNotOfferSaysWhy(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("develop")}}, Base: "develop"})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	clean := offerOf(t, detailOf(t, read, "develop").Actions, worktree.ActionClean)
	assert.Nil(t, clean.FollowUp)
	assert.Equal(t, worktree.UnofferedBaseBranch, *clean.Refused)
	assert.Equal(t, worktree.UnofferedMainWorktree, *offerOf(t, detailOf(t, read, "main").Actions, worktree.ActionClean).Refused)
}

func TestAPullRequestIsALinkToOpenNotACommand(t *testing.T) {
	offer := offerOf(t, list.Actions(list.Detail{Branch: "feat/a", PR: &list.PR{Number: 12, URL: "https://github.com/acme/acme/pull/12"}}), worktree.ActionOpenPR)

	assert.Equal(t, &domain.FollowUp{URL: "https://github.com/acme/acme/pull/12", Code: "list.open_pr", Message: "Open the pull request", Params: domain.Params{repository.ParamNumber: "12"}}, offer.FollowUp)
}

func TestAWorktreeOnNoBranchIsAnsweredByItsPath(t *testing.T) {
	followUp := offerOf(t, list.Actions(list.Detail{Path: "/repo/.trees/x"}), worktree.ActionGo).FollowUp

	require.NotNil(t, followUp)
	assert.Equal(t, "go", followUp.Command)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{"/repo/.trees/x"}}}, followUp.Answers)
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
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/orphan")}, Dirty: []string{"/repo/.trees/feat/orphan"}, Fail: map[string]error{"AheadBehindOf": errors.New("bad revision")}}})

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

func TestACountInterruptedMakesItsWorktreeCancelled(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"AheadBehindOf": context.Canceled}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, read.Items[0].Status, "the main worktree is not counted")
	assert.Equal(t, domain.StatusCancelled, read.Items[1].Status)
	assert.Equal(t, domain.KindCancelled, read.Failure().Kind)
}

func TestTheActiveWorktreeIsTheDeepestHoldingWhereTheCallerStands(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/ab")}}})
	activeIn := func(cwd string) []string {
		read, err := w.usecase.Run(t.Context(), list.Request{Caller: domain.Caller{Cwd: cwd}}, domain.RunOptions{})
		require.NoError(t, err)
		var active []string
		for _, item := range read.Items {
			if item.Detail.Active {
				active = append(active, item.Subject)
			}
		}
		return active
	}

	assert.Equal(t, []string{"feat/a"}, activeIn("/repo/.trees/feat/a/src"))
	assert.Equal(t, []string{"main"}, activeIn(mainPath))
	assert.Empty(t, activeIn("/elsewhere"))
	assert.Empty(t, activeIn(""), "a call that stands nowhere marks none")
}

func TestNoWorktreeIsActiveWhenTheOneTheCallerStandsInCannotBeRead(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Dirty": errors.New("broken index")}}})

	read, err := w.usecase.Run(t.Context(), list.Request{Caller: domain.Caller{Cwd: "/repo/.trees/feat/a/src"}}, domain.RunOptions{})

	require.NoError(t, err)
	for _, item := range read.Items {
		assert.False(t, item.Detail != nil && item.Detail.Active, "%s is not where the caller stands", item.Subject)
	}
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

func TestAWorktreeGitStoppedInTheMiddleOfAnOperationNeedsNoSyncAndSaysWhich(t *testing.T) {
	for name, ui := range map[string]worktree.Worktree{
		"rebasing":  {Path: "/repo/.trees/feat/a-ui", State: worktree.Rebasing{Branch: "feat/a-ui"}},
		"bisecting": {Path: "/repo/.trees/feat/a-ui", State: worktree.Bisecting{Branch: "feat/a-ui"}},
		"merging":   {Path: "/repo/.trees/feat/a-ui", State: worktree.OnBranch{Branch: "feat/a-ui"}, Midway: worktree.MidwayMerge},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(seed{
				Git: gittest.FakeParams{
					Worktrees: []worktree.Worktree{linked("feat/a"), ui},
					Commits: map[worktree.Ref][]string{
						worktree.LocalBranch{Name: "feat/a"}:    {"a1", "a2"},
						worktree.LocalBranch{Name: "feat/a-ui"}: {"a1", "u1"},
					},
				},
				Meta: map[string]worktree.Meta{"feat/a": {SourceBranch: "main"}, "feat/a-ui": {SourceBranch: "feat/a"}},
			})

			read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

			require.NoError(t, err)
			detail := detailOf(t, read, "feat/a-ui")
			assert.Equal(t, ptr(1), detail.Behind, "the commits it lacks are still counted")
			assert.False(t, detail.NeedsSync)
			assert.Equal(t, worktree.Condition(name), detail.Condition)
		})
	}
}

func TestAStackedWorktreeIsCountedAgainstItsParentAndARootAgainstTheBase(t *testing.T) {
	w := newWorld(seed{
		Git: gittest.FakeParams{
			Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/a-ui"), linked("feat/root")},
			Commits: map[worktree.Ref][]string{
				worktree.LocalBranch{Name: "feat/a"}:    {"a1", "a2"},
				worktree.LocalBranch{Name: "feat/a-ui"}: {"a1", "u1"},
			},
		},
		Meta: map[string]worktree.Meta{"feat/a": {SourceBranch: "main"}, "feat/a-ui": {SourceBranch: "feat/a"}},
	})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	a, ui := detailOf(t, read, "feat/a"), detailOf(t, read, "feat/a-ui")
	assert.Equal(t, "main", a.Parent)
	assert.Equal(t, ptr(2), a.Ahead)
	assert.Equal(t, ptr(0), a.Behind)
	assert.False(t, a.NeedsSync)
	root := detailOf(t, read, "feat/root")
	assert.Empty(t, root.Parent, "a worktree without a record is a root")
	assert.Equal(t, ptr(0), root.Ahead, "a root is counted against the base branch")
	assert.Nil(t, root.Behind, "a root has no parent to fall behind")
	assert.Equal(t, "feat/a", ui.Parent)
	assert.Equal(t, ptr(1), ui.Ahead)
	assert.Equal(t, ptr(1), ui.Behind)
	assert.True(t, ui.NeedsSync)
	assert.Equal(t, 2, ui.CommitsAhead, "commits_ahead stays counted against the base branch")
	main := detailOf(t, read, "main")
	assert.Nil(t, main.Ahead, "the main worktree is not counted")
	assert.Empty(t, main.Parent)
}

func TestAPullRequestMergedAtTheBranchTipMarksItsWorktreeMergedOnlyWhenAsked(t *testing.T) {
	merged := worktree.PullRequest{Number: 1, Branch: "feat/a", Head: gittest.Root, State: worktree.PullRequestMerged}
	moved := worktree.PullRequest{Number: 2, Branch: "feat/moved", Head: gittest.Root, State: worktree.PullRequestMerged}
	w := newWorld(seed{
		Git: gittest.FakeParams{
			Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/moved")},
			Commits:   map[worktree.Ref][]string{worktree.LocalBranch{Name: "feat/moved"}: {"m1"}},
		},
		PRs: []worktree.PullRequest{merged, moved},
	})

	without, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	with, err := w.usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})
	require.NoError(t, err)

	assert.False(t, detailOf(t, without, "feat/a").Merged, "GitHub not asked: not known")
	assert.True(t, detailOf(t, with, "feat/a").Merged)
	assert.False(t, detailOf(t, with, "feat/moved").Merged, "a branch that moved past its pull request is not merged")
	assert.Equal(t, worktree.ConditionClean, detailOf(t, without, "feat/a").Condition)
	assert.Equal(t, worktree.ConditionMerged, detailOf(t, with, "feat/a").Condition)
}

func TestAPullRequestMergedOrClosedAtTheBranchTipMarksItsWorktreeFinished(t *testing.T) {
	merged := worktree.PullRequest{Number: 1, Branch: "feat/a", Head: gittest.Root, State: worktree.PullRequestMerged}
	closed := worktree.PullRequest{Number: 2, Branch: "feat/b", Head: gittest.Root, State: worktree.PullRequestClosed}
	moved := worktree.PullRequest{Number: 3, Branch: "feat/moved", Head: gittest.Root, State: worktree.PullRequestClosed}
	open := worktree.PullRequest{Number: 4, Branch: "feat/open", Head: gittest.Root, State: worktree.PullRequestOpen}
	w := newWorld(seed{
		Git: gittest.FakeParams{
			Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/b"), linked("feat/moved"), linked("feat/open")},
			Commits:   map[worktree.Ref][]string{worktree.LocalBranch{Name: "feat/moved"}: {"m1"}},
		},
		PRs: []worktree.PullRequest{merged, closed, moved, open},
	})

	without, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	with, err := w.usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})
	require.NoError(t, err)

	assert.Nil(t, detailOf(t, without, "feat/a").Finished, "GitHub not asked: not known")
	assert.Equal(t, new(list.FinishedMerged), detailOf(t, with, "feat/a").Finished)
	assert.Equal(t, new(list.FinishedClosed), detailOf(t, with, "feat/b").Finished)
	assert.False(t, detailOf(t, with, "feat/b").Merged)
	assert.Nil(t, detailOf(t, with, "feat/moved").Finished, "a branch that moved past its pull request is not finished")
	assert.Nil(t, detailOf(t, with, "feat/open").Finished)
}

// prune takes a merged worktree whose directory is gone, its hooks skipped: list's finished says so, where merged, which offers clean, does not.
func TestAWorktreeWhoseDirectoryIsGoneIsFinishedButNotMerged(t *testing.T) {
	merged := worktree.PullRequest{Number: 1, Branch: "feat/a", Head: gittest.Root, State: worktree.PullRequestMerged}
	w := newWorld(seed{
		Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Gone: []string{linked("feat/a").Path}},
		PRs: []worktree.PullRequest{merged},
	})

	read, err := w.usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, detailOf(t, read, "feat/a").Missing)
	assert.False(t, detailOf(t, read, "feat/a").Merged)
	assert.Equal(t, new(list.FinishedMerged), detailOf(t, read, "feat/a").Finished)
}

func TestAnOpenPullRequestCarriesWhereItsReviewStands(t *testing.T) {
	open := worktree.PullRequest{Number: 3, URL: "https://github.com/acme/acme/pull/3", Branch: "feat/a", State: worktree.PullRequestOpen, Title: "Add a", Draft: true, Checks: worktree.Checks{Passed: 2, Pending: 1}, Decision: "APPROVED"}
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, PRs: []worktree.PullRequest{open}})

	read, err := w.usecase.Run(t.Context(), list.Request{WithPRs: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, &list.PR{Number: 3, URL: open.URL, State: worktree.PullRequestOpen, Progress: &worktree.Progress{Title: "Add a", Draft: true, Checks: worktree.Checks{Passed: 2, Pending: 1}, Decision: "APPROVED"}}, detailOf(t, read, "feat/a").PR)
}

func TestEachWorktreeSaysWhenItsBranchLastMoved(t *testing.T) {
	moved := created.Add(48 * time.Hour)
	w := newWorld(seed{Git: gittest.FakeParams{
		Worktrees: []worktree.Worktree{linked("feat/a"), {Path: "/repo/.trees/detached", State: worktree.Detached{}}},
		Dates:     map[string]time.Time{"feat/a": moved},
	}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, &moved, detailOf(t, read, "feat/a").LastCommitAt)
	assert.Nil(t, detailOf(t, read, "/repo/.trees/detached").LastCommitAt, "a worktree on no branch has no branch to date")
}

func TestAMissingWorktreeHasNoChangesToCount(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/b")}, Gone: []string{"/repo/.trees/feat/a"}}})

	read, err := w.usecase.Run(t.Context(), list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, detailOf(t, read, "feat/a").Changes)
	assert.Equal(t, &list.Changes{}, detailOf(t, read, "feat/b").Changes)
}

func TestAListCancelledWarnsOfNoDatesItCouldNotRead(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"CommitDates": context.Canceled}}})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	read, err := w.usecase.Run(ctx, list.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, read.Interrupted)
	assert.NotContains(t, codes(read), list.CodeDatesUnreadable, "a cancelled read is said once, as interrupted")
}
