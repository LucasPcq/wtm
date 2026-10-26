package status_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeIsReadWholeGitAndIsolation(t *testing.T) {
	exit := 1
	w := newWorld(seed{
		Git: gittest.FakeParams{
			Remote:    []string{"main", "feat/a"},
			Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/b")},
			Commits: map[worktree.Ref][]string{
				worktree.LocalBranch{Name: "feat/a"}: {"one", "two"},
				worktree.LocalBranch{Name: "main"}:   {"upstream"},
			},
			Changes:   map[string][]worktree.Change{"/repo/.trees/feat/a": {{Path: "a.go", Status: worktree.ChangeModified}, {Path: "new.go", Status: worktree.ChangeUntracked}}},
			Diffstats: map[string]worktree.Diffstat{"/repo/.trees/feat/a": {Staged: 1, Unstaged: 1, Insertions: 12, Deletions: 3}},
		},
		Meta: map[string]worktree.Meta{
			"feat/a": {CreatedAt: createdAt, SourceBranch: "main", Ordinal: 1, Isolation: project.Isolated},
			"feat/b": {CreatedAt: createdAt, SourceBranch: "feat/a", Ordinal: 2, Isolation: project.Isolated},
		},
		Run:  isolating(),
		Jobs: []run.Job{{Name: "web", WorkDir: "/repo/.trees/feat/a", Up: true, State: run.Crashed, ExitCode: &exit, URL: "http://web.feat-a.main.localhost:11080"}},
		PRs:  []worktree.PullRequest{{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/a", State: worktree.PullRequestOpen, Title: "Add a", Checks: worktree.Checks{Passed: 2, Failed: 1}, Decision: "APPROVED"}},
	})

	got := read(t, w, status.Request{Worktree: "feat/a", WithPRs: true})

	a := only(t, got)
	at, _ := time.Parse(time.RFC3339, createdAt)
	isolated, offset, port := project.Isolated, 10, 3010
	assert.Equal(t, status.Detail{
		Branch: "feat/a", Path: "/repo/.trees/feat/a", CreatedAt: &at,
		Parent: "main", Children: []string{"feat/b"},
		Base:      &status.Base{Name: "main", Ahead: 2, Behind: 1},
		Origin:    &worktree.Origin{Ahead: 2, State: worktree.Ahead},
		Changes:   &status.Changes{Staged: 1, Modified: 1, Untracked: 1, Insertions: 12, Deletions: 3, Files: []worktree.Change{{Path: "a.go", Status: worktree.ChangeModified}, {Path: "new.go", Status: worktree.ChangeUntracked}}},
		Commits:   []worktree.Commit{{SHA: "two", Subject: "two"}, {SHA: "one", Subject: "one"}, {SHA: gittest.Root, Subject: gittest.Root}},
		Review:    &status.Review{Number: 12, URL: "https://github.com/acme/acme/pull/12", State: worktree.PullRequestOpen, Progress: &worktree.Progress{Title: "Add a", Checks: worktree.Checks{Passed: 2, Failed: 1}, Decision: "APPROVED"}},
		Isolation: &isolated,
		RunToml:   status.RunTomlDeclared,
		Run:       &status.Run{Addressing: project.AddressNames, Offset: &offset, Ports: status.PortsOffset},
		Jobs:      []status.Job{{Name: "web", State: run.Crashed, URL: "http://web.feat-a.main.localhost:11080", Port: &port, ExitCode: &exit}},
		Env:       status.Env{Files: []status.EnvFile{}},
		Problems: []status.Problem{{Code: status.CodeBehind, Message: "main has 1 commit it lacks", Fix: &domain.FollowUp{Command: "sync", Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}}, Code: status.CodeBehind, Message: "sync it onto main"}}, {Code: status.CodeJobCrashed, Message: "web crashed (exit 1)", Fix: &domain.FollowUp{
			Command: "run start", Code: status.CodeJobCrashed, Message: "start it again",
			Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}, {Path: "job", Values: []string{"web"}}, {Path: "detach", Values: []string{"true"}}},
		}}},
		Condition: worktree.ConditionChanged,
	}, a)
	assert.Empty(t, got.Warnings)
}

func TestAReadChangesNothingAndTakesNoLock(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, Meta: map[string]worktree.Meta{"feat/a": {CreatedAt: createdAt}}})

	read(t, w, status.Request{All: true})

	for _, call := range append(w.git.Calls(), w.store.Calls()...) {
		assert.Regexp(t, `^(Worktrees|Branches|AheadBehind|Changes|Diffstat|Commits|AheadBehindOf|Load|Read|Locate) `, call)
	}
}

func TestWithNoWorktreeNamedTheOneHoldingTheDirectoryIsReadElseTheMainOne(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{
		linked("feat/a"),
		{Path: "/repo/.trees/rebasing", State: worktree.Rebasing{Branch: "fix/rebase"}},
	}}})

	cases := map[string]struct{ cwd, read string }{
		"inside_a_worktree": {cwd: "/repo/.trees/feat/a/src", read: "feat/a"},
		"rebasing":          {cwd: "/repo/.trees/rebasing", read: "fix/rebase"},
		"outside":           {cwd: "/elsewhere", read: "main"},
		"no_cwd":            {read: "main"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := read(t, w, status.Request{Caller: domain.Caller{Cwd: c.cwd}})

			require.Len(t, got.Items, 1)
			assert.Equal(t, c.read, got.Items[0].Subject)
		})
	}
	assert.True(t, only(t, read(t, w, status.Request{Caller: domain.Caller{Cwd: "/repo/.trees/rebasing"}})).Rebasing, "a worktree mid-rebase is read on its branch, never refused as detached")
}

func TestAWorktreeIsNamedByItsBranchItsPathOrAPartOfItsBranch(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/login"), linked("feat/logout"), {Path: "/repo/.trees/detached", State: worktree.Detached{}}}}})

	cases := map[string]struct{ named, cwd, read string }{
		"branch":        {named: "feat/login", read: "feat/login"},
		"path":          {named: "/repo/.trees/detached", read: "/repo/.trees/detached"},
		"relative_path": {named: "../.trees/feat/logout", cwd: mainPath, read: "feat/logout"},
		"part":          {named: "logout", read: "feat/logout"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := read(t, w, status.Request{Worktree: c.named, Caller: domain.Caller{Cwd: c.cwd}})

			require.Len(t, got.Items, 1)
			assert.Equal(t, c.read, got.Items[0].Subject)
		})
	}
}

func TestANameMatchingNoWorktreeOrSeveralIsRefused(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/login"), linked("feat/logout")}}})

	cases := map[string]struct {
		req  status.Request
		code string
	}{
		"not_found":         {req: status.Request{Worktree: "nope"}, code: domain.CodeNotFound},
		"ambiguous":         {req: status.Request{Worktree: "log"}, code: domain.CodeAmbiguous},
		"all_with_worktree": {req: status.Request{Worktree: "feat/login", All: true}, code: status.CodeAllWithWorktree},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := w.usecase.Run(t.Context(), c.req, domain.RunOptions{})

			failure, ok := errors.AsType[*domain.Error](err)
			require.True(t, ok, err)
			assert.Equal(t, domain.KindInvalid, failure.Kind)
			require.Len(t, failure.Fields, 1)
			assert.Equal(t, c.code, failure.Fields[0].Code)
		})
	}
}

func TestAllReadsEveryWorktreeAndOneThatCannotBeReadFailsAlone(t *testing.T) {
	broken := errors.New("broken index")
	w := newWorld(seed{Git: gittest.FakeParams{
		Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/gone"), {Path: "/repo/.trees/detached", State: worktree.Detached{}}},
		Gone:      []string{"/repo/.trees/feat/gone"},
	}})
	w.usecase.Git = failingChanges{Git: w.usecase.Git, path: "/repo/.trees/feat/a", err: broken}

	got := read(t, w, status.Request{All: true})

	require.Len(t, got.Items, 4)
	var subjects []string
	var statuses []domain.Status
	for _, item := range got.Items {
		subjects, statuses = append(subjects, item.Subject), append(statuses, item.Status)
	}
	assert.Equal(t, []string{"main", "/repo/.trees/detached", "feat/a", "feat/gone"}, subjects, "the main worktree first, then as git lists them; one on no branch named by its path")
	assert.Equal(t, []domain.Status{domain.StatusDone, domain.StatusDone, domain.StatusFailed, domain.StatusDone}, statuses)
	assert.Equal(t, status.CodeUnreadable, got.Items[2].Err.Code)
	assert.ErrorIs(t, got.Items[2].Err, broken)
}

func TestAWorktreeWhoseDirectoryIsGoneIsMissingAndToldToBeCleanedLockedOrNot(t *testing.T) {
	locked := linked("feat/locked")
	locked.Locked = true
	prunable := linked("feat/prunable")
	prunable.Prunable = true
	detached := worktree.Worktree{Path: "/repo/.trees/detached", State: worktree.Detached{}, Locked: true}
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{locked, prunable, detached}, Gone: []string{locked.Path, prunable.Path, detached.Path}}})

	got := read(t, w, status.Request{All: true})

	assert.Nil(t, got.Failure(), "a gone directory is found, not a read that failed")
	for _, branch := range []string{"feat/locked", "feat/prunable"} {
		d := detailOf(t, got, branch)
		assert.True(t, d.Missing, branch)
		assert.Equal(t, []string{status.CodeMissing}, problemCodes(d.Problems), branch)
		assert.Equal(t, &domain.FollowUp{Command: domain.CommandClean, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}}, Code: status.CodeMissing, Message: "remove what is left of it"}, d.Problems[0].Fix)
		assert.Nil(t, d.Changes, branch)
		assert.Nil(t, d.Commits, branch)
		assert.Nil(t, d.Env.Files, branch)
	}
	gone := detailOf(t, got, detached.Path)
	assert.True(t, gone.Missing)
	require.Len(t, gone.Problems, 1, "a gone worktree on no branch is a problem too")
	assert.Equal(t, status.CodeMissing, gone.Problems[0].Code)
	assert.Equal(t, &domain.FollowUp{Command: domain.CommandClean, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{detached.Path}}}, Code: status.CodeMissing, Message: "remove what is left of it"}, gone.Problems[0].Fix, "clean removes a worktree on no branch by its path")
	assert.NotContains(t, w.git.Calls(), "Changes "+locked.Path, "nothing of a gone directory is read")
}

func TestAWorktreeThatHasNotChosenItsIsolationHasNoneAndIsToldHowToChoose(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/old")}}, Meta: map[string]worktree.Meta{"feat/old": {CreatedAt: createdAt}}, Run: isolating()})

	old := only(t, read(t, w, status.Request{Worktree: "feat/old"}))

	assert.Nil(t, old.Isolation, "the legacy reported isolated for a worktree that never chose")
	assert.Equal(t, []string{status.CodeIsolationPending}, problemCodes(old.Problems))
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/old"}}, {Path: domain.PathIsolation, Values: []string{"isolated"}}}, old.Problems[0].Fix.Answers)
	main := only(t, read(t, w, status.Request{}))
	assert.Equal(t, project.Isolated, *main.Isolation, "the main worktree never chooses")
	assert.Empty(t, main.Problems)
}

func TestTheRunOfAWorktreeReflectsRunToml(t *testing.T) {
	malformed := errors.New("invalid run config: unknown addressing \"nope\"")
	cases := map[string]struct {
		run      project.Run
		runErr   error
		want     *status.Run
		warnings []string
	}{
		"absent":           {},
		"declared_no_jobs": {run: project.Run{Declared: true, Addressing: project.AddressPorts}, want: &status.Run{Addressing: project.AddressPorts, Offset: new(int), Ports: status.PortsBase}},
		"malformed":        {runErr: malformed, warnings: []string{status.CodeRunInvalid}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(seed{Run: c.run, RunErr: c.runErr})

			got := read(t, w, status.Request{})

			assert.Equal(t, c.want, only(t, got).Run)
			assert.Equal(t, c.warnings, codes(got.Warnings), "a run.toml that does not read is a warning, never a refusal")
		})
	}
}

func TestOnlyAnUnnumberedWorktreeHasNoOffsetAndARecordThatDoesNotReadIsAWarning(t *testing.T) {
	w := newWorld(seed{
		Git:        gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/b"), linked("feat/c"), linked("feat/d")}},
		Meta:       map[string]worktree.Meta{"feat/a": {Ordinal: 1, Isolation: project.Isolated}, "feat/b": {Ordinal: 1, Isolation: project.Isolated}, "feat/c": {Isolation: project.Isolated}},
		Unreadable: []string{"feat/d"},
		Run:        isolating(),
	})

	got := read(t, w, status.Request{All: true})

	assert.Equal(t, 10, *detailOf(t, got, "feat/a").Run.Offset)
	assert.Nil(t, detailOf(t, got, "feat/b").Run.Offset, "an ordinal another branch holds first is not this one's")
	assert.Nil(t, detailOf(t, got, "feat/c").Run.Offset, "no run has numbered it")
	assert.Nil(t, detailOf(t, got, "feat/d").Run.Offset)
	assert.Equal(t, []string{status.CodeMetaUnreadable}, codes(got.Items[4].Warnings), "an unreadable record is said, not taken for an unnumbered worktree")
	d := detailOf(t, got, "feat/d")
	assert.Empty(t, d.Problems, "a record that does not read is no isolation to choose: its fix would write over it")
	assert.Nil(t, d.Isolation)
	assert.Equal(t, status.PortsUnknown, d.Run.Ports)
}

func TestARecordThatDoesNotReadIsAWarningOnAWorktreeNotReadToo(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, Unreadable: []string{"feat/a"}})
	w.usecase.Git = failingChanges{Git: w.usecase.Git, path: "/repo/.trees/feat/a", err: errors.New("broken index")}

	got := read(t, w, status.Request{Worktree: "feat/a"})

	require.Len(t, got.Items, 1)
	assert.Equal(t, domain.StatusFailed, got.Items[0].Status)
	assert.Equal(t, []string{status.CodeMetaUnreadable}, codes(got.Items[0].Warnings))
}

func TestASharedJobAWorktreeHoldsReadsAsItsOwnersInstanceOnce(t *testing.T) {
	exit := 2
	w := newWorld(seed{
		Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}},
		Run: project.Run{Declared: true, Jobs: []project.Job{{Name: "db", Kind: project.Service, Scope: project.Shared}}},
		Jobs: []run.Job{
			{Name: "db", WorkDir: mainPath, State: run.Crashed, ExitCode: &exit, URL: "http://db.main.localhost", SharedDir: mainPath},
			{Name: "db", WorkDir: "/repo/.trees/feat/a", Claim: true, Up: true, URL: "http://stale.localhost", SharedDir: mainPath},
			{Name: "db", WorkDir: "/repo/.trees/feat/a", Claim: true, Up: true, SharedDir: mainPath},
		},
	})

	a := only(t, read(t, w, status.Request{Worktree: "feat/a"}))

	require.Len(t, a.Jobs, 1, "two claims on one shared job are one job")
	assert.Equal(t, status.Job{Name: "db", State: run.Crashed, URL: "http://db.main.localhost", ExitCode: &exit, Shared: true, Owner: &status.Owner{Branch: "main", Path: mainPath}}, a.Jobs[0])
	require.Len(t, a.Problems, 1)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}, {Path: "job", Values: []string{"db"}}, {Path: "detach", Values: []string{"true"}}}, a.Problems[0].Fix.Answers, "a crashed shared job starts again where it runs")
}

func TestAJobRunTomlNoLongerDeclaresIsListedLastOnlyWhileItRuns(t *testing.T) {
	w := newWorld(seed{
		Run: isolating(),
		Jobs: []run.Job{
			{Name: "old", WorkDir: mainPath, State: run.Running, Up: true},
			{Name: "gone", WorkDir: mainPath, State: run.Stopped},
		},
	})

	main := only(t, read(t, w, status.Request{}))

	assert.Equal(t, []string{"web", "old"}, []string{main.Jobs[0].Name, main.Jobs[1].Name})
	assert.Equal(t, run.Stopped, main.Jobs[0].State, "a declared job nothing started is stopped")
	assert.Len(t, main.Jobs, 2)
}

func TestWhatCannotBeAskedIsNullAndAWarning(t *testing.T) {
	w := newWorld(seed{
		JobsErr: run.ErrUnknown,
		PRsErr:  errors.Join(worktree.ErrPullRequestsUnavailable, errors.New("run `gh auth login`")),
	})

	got := read(t, w, status.Request{WithPRs: true})

	main := only(t, got)
	assert.Nil(t, main.Jobs)
	assert.Nil(t, main.Review)
	assert.Equal(t, []string{status.CodeJobsUnknown, status.CodePRsUnavailable}, codes(got.Warnings))
}

func TestAnInterruptedReadWarnsOfNothingItCouldNotAsk(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	w := newWorld(seed{JobsErr: run.ErrUnknown, PRsErr: context.Canceled})

	got, err := w.usecase.Run(ctx, status.Request{WithPRs: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Empty(t, got.Warnings)
	assert.Equal(t, domain.KindCancelled, got.Failure().Kind, "every worktree read, the read still ends interrupted")
}

func TestTheReviewIsTheNewestPullRequestAndAMergedOneAsksToCleanFirst(t *testing.T) {
	const url = "https://github.com/acme/acme/pull/12"
	clean := &domain.FollowUp{Command: domain.CommandClean, Code: status.CodeMerged, Message: "clean it up", Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}}}
	cases := map[string]struct {
		pr       []worktree.PullRequest
		review   *status.Review
		problems []status.Problem
	}{
		"open": {
			pr:       []worktree.PullRequest{{Number: 12, URL: url, Branch: "feat/a", State: worktree.PullRequestOpen, Title: "Add a", Draft: true, Checks: worktree.Checks{Pending: 1}, Decision: "REVIEW_REQUIRED"}},
			review:   &status.Review{Number: 12, URL: url, State: worktree.PullRequestOpen, Progress: &worktree.Progress{Title: "Add a", Draft: true, Checks: worktree.Checks{Pending: 1}, Decision: "REVIEW_REQUIRED"}},
			problems: []status.Problem{{Code: status.CodeEnvMissing, Message: ".env is missing", Fix: fixOf("env", domain.Answered{Path: domain.PathWorktree, Values: []string{"feat/a"}})}},
		},
		"merged": {
			pr:     []worktree.PullRequest{{Number: 12, URL: url, Branch: "feat/a", Head: gittest.Root, State: worktree.PullRequestMerged, Title: "Add a"}},
			review: &status.Review{Number: 12, URL: url, State: worktree.PullRequestMerged},
			problems: []status.Problem{
				{Code: status.CodeMerged, Message: "its pull request is merged", Fix: clean},
				{Code: status.CodeEnvMissing, Message: ".env is missing", Fix: fixOf("env", domain.Answered{Path: domain.PathWorktree, Values: []string{"feat/a"}})},
			},
		},
		"closed": {
			pr:       []worktree.PullRequest{{Number: 12, URL: url, Branch: "feat/a", State: worktree.PullRequestClosed}},
			review:   &status.Review{Number: 12, URL: url, State: worktree.PullRequestClosed},
			problems: []status.Problem{{Code: status.CodeEnvMissing, Message: ".env is missing", Fix: fixOf("env", domain.Answered{Path: domain.PathWorktree, Values: []string{"feat/a"}})}},
		},
		"none": {
			problems: []status.Problem{{Code: status.CodeEnvMissing, Message: ".env is missing", Fix: fixOf("env", domain.Answered{Path: domain.PathWorktree, Values: []string{"feat/a"}})}},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(seed{
				Git:      gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}},
				EnvFiles: []project.EnvFile{{Target: ".env"}},
				Strategy: project.EnvMain,
				Files:    map[string]string{mainPath + "/.env": "A=1\n"},
				PRs:      c.pr,
			})

			a := only(t, read(t, w, status.Request{Worktree: "feat/a", WithPRs: true}))

			assert.Equal(t, c.review, a.Review)
			assert.Equal(t, c.problems, a.Problems)
		})
	}
}

func TestAMergedPullRequestOfTheMainWorktreeOrTheBaseBranchIsNoProblem(t *testing.T) {
	merged := func(branch string) worktree.PullRequest {
		return worktree.PullRequest{Number: 1, Branch: branch, Head: gittest.Root, State: worktree.PullRequestMerged}
	}
	cases := map[string]struct {
		git     gittest.FakeParams
		subject string
	}{
		"main_worktree": {git: gittest.FakeParams{Main: mainPath, Branch: "trunk"}, subject: "trunk"},
		"base_branch":   {git: gittest.FakeParams{Main: mainPath, Branch: "trunk", Worktrees: []worktree.Worktree{linked("main")}}, subject: "main"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(seed{Git: c.git, PRs: []worktree.PullRequest{merged("trunk"), merged("main")}})

			got := read(t, w, status.Request{Worktree: c.subject, WithPRs: true})

			assert.Equal(t, worktree.PullRequestMerged, only(t, got).Review.State)
			assert.Empty(t, only(t, got).Problems)
		})
	}
}

func TestAMergedPullRequestAsksToCleanABranchBehindItsHeadButNotOneMovedOnOrApart(t *testing.T) {
	clean := []string{status.CodeMerged}
	cases := map[string]struct {
		head     string
		problems []string
	}{
		"behind":       {head: "a2", problems: clean},
		"moved_on":     {head: gittest.Root},
		"never_pulled": {head: "a commit never fetched"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(seed{
				Git: gittest.FakeParams{
					Remote:    []string{"feat/a"},
					Worktrees: []worktree.Worktree{linked("feat/a")},
					Commits: map[worktree.Ref][]string{
						worktree.LocalBranch{Name: "feat/a"}:  {"a1"},
						worktree.RemoteBranch{Name: "feat/a"}: {"a1", "a2"},
					},
				},
				PRs: []worktree.PullRequest{{Number: 12, Branch: "feat/a", Head: c.head, State: worktree.PullRequestMerged}},
			})

			a := only(t, read(t, w, status.Request{Worktree: "feat/a", WithPRs: true}))

			assert.Equal(t, worktree.PullRequestMerged, a.Review.State)
			assert.Equal(t, c.problems, problemCodes(a.Problems))
			if c.problems != nil {
				assert.Equal(t, &domain.FollowUp{Command: domain.CommandClean, Code: status.CodeMerged, Message: "clean it up", Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/a"}}}}, a.Problems[0].Fix)
			}
		})
	}
}

func TestGitHubIsAskedOfTheBranchesReadOnly(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/b"), {Path: "/repo/.trees/detached", State: worktree.Detached{}}}}})

	read(t, w, status.Request{Worktree: "feat/a", WithPRs: true})
	read(t, w, status.Request{All: true, WithPRs: true})

	assert.Equal(t, [][]string{{"feat/a"}, {"main", "", "feat/a", "feat/b"}}, *w.asked)
}

func TestWhatGitHubFoundBeforeFailingIsKept(t *testing.T) {
	w := newWorld(seed{
		Git:    gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}},
		PRs:    []worktree.PullRequest{{Number: 12, Branch: "feat/a", State: worktree.PullRequestClosed}},
		PRsErr: errors.Join(worktree.ErrPullRequestsUnavailable, errors.New("boom")),
	})

	got := read(t, w, status.Request{All: true, WithPRs: true})

	assert.Equal(t, 12, detailOf(t, got, "feat/a").Review.Number)
	assert.Equal(t, []string{status.CodePRsUnavailable}, codes(got.Warnings))
}

func TestAMissingEnvFileIsRebuiltFromWhereItCanBe(t *testing.T) {
	file := project.EnvFile{Target: ".env"}
	cases := map[string]struct {
		strategy project.EnvStrategy
		files    map[string]string
		message  string
		fix      *domain.FollowUp
	}{
		"from_its_source": {
			strategy: project.EnvMain, files: map[string]string{mainPath + "/.env": "A=1\n"},
			message: ".env is missing", fix: fixOf("env", domain.Answered{Path: domain.PathWorktree, Values: []string{"feat/a"}}),
		},
		"from_the_template": {
			strategy: project.EnvMain, files: map[string]string{"/repo/.trees/feat/a/.env.example": "A=\n"},
			message: ".env is missing", fix: fixOf("env", domain.Answered{Path: domain.PathWorktree, Values: []string{"feat/a"}}, domain.Answered{Path: "from", Values: []string{"example"}}),
		},
		"example_with_no_template_from_main": {
			strategy: project.EnvExample, files: map[string]string{mainPath + "/.env": "A=1\n"},
			message: ".env is missing", fix: fixOf("env", domain.Answered{Path: domain.PathWorktree, Values: []string{"feat/a"}}, domain.Answered{Path: "from", Values: []string{"main"}}),
		},
		"nowhere": {
			strategy: project.EnvExample,
			message:  ".env is missing, with no copy to rebuild it from and no template: declare it where it exists, or write it by hand", fix: &domain.FollowUp{Command: "config edit", Code: status.CodeEnvMissing, Message: "declare where it comes from"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}}, EnvFiles: []project.EnvFile{file}, Strategy: c.strategy, Files: c.files})

			a := only(t, read(t, w, status.Request{Worktree: "feat/a"}))

			assert.Equal(t, status.Env{Declared: 1, Files: []status.EnvFile{{Target: ".env", Missing: true}}}, a.Env)
			assert.Equal(t, []status.Problem{{Code: status.CodeEnvMissing, Message: c.message, Fix: c.fix}}, a.Problems)
		})
	}
}

func TestAnEnvFileCountsHowItsKeysStandAgainstItsSource(t *testing.T) {
	w := newWorld(seed{
		Git:      gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}},
		EnvFiles: []project.EnvFile{{Target: ".env"}},
		Strategy: project.EnvMain,
		Files: map[string]string{
			mainPath + "/.env":                 "A=1\nB=2\n",
			"/repo/.trees/feat/a/.env.example": "A=\nB=\nSECRET=\n",
			"/repo/.trees/feat/a/.env":         "A=1\nB=3\nLOCAL=x\n",
		},
	})

	a := only(t, read(t, w, status.Request{Worktree: "feat/a"}))

	assert.Equal(t, []status.EnvFile{{Target: ".env", Keys: env.Drift{Missing: 1, Conflicting: 1, Orphan: 1}}}, a.Env.Files)
	assert.Empty(t, a.Problems)
}

func TestAWorktreeOnNoBranchIsReadWithNothingToFixSinceEveryFixNamesABranch(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{{Path: "/repo/.trees/detached", State: worktree.Detached{}}}}, EnvFiles: []project.EnvFile{{Target: ".env"}}, Run: isolating()})

	detached := only(t, read(t, w, status.Request{Worktree: "/repo/.trees/detached"}))

	assert.Equal(t, []status.EnvFile{{Target: ".env", Missing: true}}, detached.Env.Files)
	assert.Empty(t, detached.Problems)
	assert.NotNil(t, detached.Problems)
}

func TestAPathNamesTheDeepestWorktreeHoldingItAndABranchNameComesFirst(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{
		Worktrees: []worktree.Worktree{linked("feat/a")},
		Folders:   []string{"/repo/.trees/feat/a/src", "/repo/.trees/feat/a/main"},
	}})

	cases := map[string]struct{ named, cwd, read string }{
		"dot_in_a_subdirectory": {named: ".", cwd: "/repo/.trees/feat/a/src", read: "feat/a"},
		"branch_over_path":      {named: "main", cwd: "/repo/.trees/feat/a", read: "main"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := read(t, w, status.Request{Worktree: c.named, Caller: domain.Caller{Cwd: c.cwd}})

			require.Len(t, got.Items, 1)
			assert.Equal(t, c.read, got.Items[0].Subject)
		})
	}
}

func TestARepositoryWithNoWorktreeIsRefusedUnlessAllAreRead(t *testing.T) {
	bare := worktree.Snapshot{Worktrees: []worktree.Worktree{{Path: "/repo.git", Main: true, State: worktree.Bare{}}}}

	_, err := status.Targets(bare, status.Request{}, "")
	all, allErr := status.Targets(bare, status.Request{All: true}, "")

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, status.CodeNoWorktree, failure.Code)
	require.NoError(t, allErr)
	assert.Empty(t, all)
}

func TestAReadCancelledReportsItsWorktreeInterruptedAndWhatDidNotReadIsSaid(t *testing.T) {
	cancelled := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Diffstat": context.Canceled}}})
	partial := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"Commits": errors.New("no commit yet")}}})

	stopped := read(t, cancelled, status.Request{All: true})
	half := read(t, partial, status.Request{Worktree: "feat/a"})

	for _, item := range stopped.Items {
		assert.Equal(t, domain.StatusCancelled, item.Status, item.Subject)
		assert.Equal(t, domain.ReasonInterrupted, item.Reason, item.Subject)
	}
	assert.Equal(t, domain.KindCancelled, stopped.Failure().Kind)
	require.Len(t, half.Items, 1)
	assert.Equal(t, domain.StatusDone, half.Items[0].Status)
	assert.Equal(t, []string{status.CodePartial}, codes(half.Items[0].Warnings))
	assert.Contains(t, half.Items[0].Warnings[0].Message, "its commits: no commit yet")
}

func TestAWorktreeBehindItsParentIsToldToSyncOntoIt(t *testing.T) {
	cases := map[string]struct {
		meta  map[string]worktree.Meta
		want  []string
		lacks string
	}{
		"child_of_a_worktree": {meta: map[string]worktree.Meta{"feat/a": {SourceBranch: "main"}, "feat/a-ui": {SourceBranch: "feat/a"}}, want: []string{status.CodeBehind}, lacks: "feat/a has 2 commits it lacks"},
		"root":                {want: []string{}},
		"in_step":             {meta: map[string]worktree.Meta{"feat/a-ui": {SourceBranch: "feat/a-ui-base"}}, want: []string{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(seed{
				Git: gittest.FakeParams{
					Worktrees: []worktree.Worktree{linked("feat/a"), linked("feat/a-ui")},
					Local:     []string{"feat/a-ui-base"},
					Commits: map[worktree.Ref][]string{
						worktree.LocalBranch{Name: "feat/a"}:         {"a1", "a2"},
						worktree.LocalBranch{Name: "feat/a-ui-base"}: {"u1"},
						worktree.LocalBranch{Name: "feat/a-ui"}:      {"u1", "u2"},
					},
				},
				Meta: c.meta,
			})

			ui := only(t, read(t, w, status.Request{Worktree: "feat/a-ui"}))

			codes := []string{}
			for _, problem := range ui.Problems {
				codes = append(codes, problem.Code)
			}
			assert.Equal(t, c.want, codes)
			if c.lacks != "" {
				assert.Equal(t, c.lacks, ui.Problems[0].Message)
				assert.Equal(t, "sync", ui.Problems[0].Fix.Command)
			}
		})
	}
}

func TestEachWorktreeCarriesItsConditionTheFirstThatApplies(t *testing.T) {
	merged := linked("feat/merged")
	w := newWorld(seed{
		Git: gittest.FakeParams{
			Worktrees: []worktree.Worktree{linked("feat/clean"), linked("feat/changed"), merged, linked("feat/gone")},
			Changes:   map[string][]worktree.Change{"/repo/.trees/feat/changed": {{Path: "a.go", Status: worktree.ChangeUntracked}}, merged.Path: {{Path: "b.go", Status: worktree.ChangeUntracked}}},
			Gone:      []string{"/repo/.trees/feat/gone"},
		},
		PRs: []worktree.PullRequest{{Number: 1, Branch: "feat/merged", Head: gittest.Root, State: worktree.PullRequestMerged}},
	})

	got := read(t, w, status.Request{All: true, WithPRs: true})

	for branch, want := range map[string]worktree.Condition{
		"feat/clean":   worktree.ConditionClean,
		"feat/changed": worktree.ConditionChanged,
		"feat/merged":  worktree.ConditionMerged,
		"feat/gone":    worktree.ConditionMissing,
	} {
		assert.Equal(t, want, detailOf(t, got, branch).Condition, branch)
	}
}

func TestWhereABranchStandsAgainstTheBaseIsAskedOnceAndSaidUnreadOnceWhenItFails(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/a")}, Fail: map[string]error{"AheadBehindOf": errors.New("rev-list failed")}}})

	got := read(t, w, status.Request{Worktree: "feat/a"})

	require.Len(t, got.Items, 1)
	assert.Equal(t, []string{status.CodePartial}, codes(got.Items[0].Warnings))
	assert.Equal(t, 1, strings.Count(got.Items[0].Warnings[0].Message, "where it stands against main"), got.Items[0].Warnings[0].Message)
	asked := 0
	for _, call := range w.git.Calls() {
		if strings.HasPrefix(call, "AheadBehindOf ") {
			asked++
		}
	}
	assert.Equal(t, 1, asked, "a count that failed is not asked again")
}
