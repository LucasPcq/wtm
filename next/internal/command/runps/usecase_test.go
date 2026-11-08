package runps_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryRepositoryOfTheMachineIsListedEachWithItsCrashesFirstAndItsSharedServicesLast(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme(), other()}}, runps.Request{})

	assert.Equal(t, []string{"feat/billing", "fix/login", "shared", "main"}, subjects(read))
	assert.Equal(t, otherPath, group(t, read, "main").Repository)
	assert.Empty(t, read.Warnings)
}

func TestAGroupListsWhatCrashedFirstThenWhatRunsThenWhatEndedOrNeverStarted(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme()}}, runps.Request{})

	billing := group(t, read, "feat/billing")
	assert.Equal(t, []string{"worker:crashed", "web:running", "migrate:exited", "api:not_started"}, names(billing.Jobs), "an ended task is listed, and the profile's job the daemon does not list too")
	assert.Equal(t, runps.Counts{Running: 1, Crashed: 1, Exited: 1, NotStarted: 1}, billing.Counts)
	assert.Equal(t, "dev", billing.Profile)
	assert.Equal(t, "feat/billing", billing.Branch)
	assert.Equal(t, billingPath, billing.Path)
}

func TestAJobSaysWhereItAnswersWhereItRunsAndWhereItsLogIs(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme()}}, runps.Request{})

	web := group(t, read, "feat/billing").Jobs[1]
	assert.Equal(t, "localhost:3020", web.Address)
	assert.Equal(t, "http://web.feat-billing.acme.localhost", web.URL)
	assert.Equal(t, "apps/web", web.Dir)
	assert.Equal(t, "pnpm dev", web.Command)
	assert.Equal(t, "/state/logs/feat%2Fbilling/web.log", web.Log)
	assert.Equal(t, &runps.Line{At: ended, Text: "GET / 200"}, web.LastLine)
	assert.Nil(t, web.EndedAt)
}

func TestAJobThatEndedSaysWhenAndHowLongItRan(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme()}}, runps.Request{})

	worker := group(t, read, "feat/billing").Jobs[0]
	require.NotNil(t, worker.EndedAt)
	assert.Equal(t, ended, *worker.EndedAt)
	assert.Equal(t, int64((10 * time.Minute).Milliseconds()), *worker.DurationMS)
	assert.Equal(t, 1, *worker.ExitCode)
}

func TestADeclaredJobTheDaemonDoesNotListIsStoppedOnceItsLogSaysItRan(t *testing.T) {
	last := run.Line{At: started, Text: "api stopping"}
	s := seed{Jobs: jobs(), Projects: []runps.Project{acme()}, Logged: map[run.LogFile]run.Line{{Dir: "/state/logs/feat%2Fbilling", Job: "api"}: last}}

	api := group(t, read(t, s, runps.Request{}), "feat/billing").Jobs[3]

	assert.Equal(t, "api:stopped", names([]runps.Job{api})[0])
	assert.Equal(t, &runps.Line{At: started, Text: "api stopping"}, api.LastLine)
}

func TestARunnerListsTheServicesItStartsAtItsWorktreesPorts(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme()}}, runps.Request{})

	login := group(t, read, "fix/login")
	assert.Equal(t, []string{"web:running", "stack:running"}, names(login.Jobs))
	assert.Equal(t, []runps.Service{{Name: "mailpit", Address: "localhost:8055"}}, login.Jobs[1].Services, "fix/login's ordinal 3 puts its ports 30 above main's")
}

func TestASharedServiceListsTheWorktreesHoldingItTheirNamespaceAndSinceWhen(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme()}}, runps.Request{})

	shared := group(t, read, "shared")
	require.Len(t, shared.Jobs, 1)
	db := shared.Jobs[0]
	assert.Equal(t, "localhost:5432", db.Address)
	since := func(d time.Duration) *time.Time { return new(started.Add(d)) }
	assert.Equal(t, []runps.Holder{
		{Branch: "main", Path: mainPath, Namespace: "acme_main", Since: since(0)},
		{Branch: "fix/login", Path: loginPath, Namespace: "acme_fix_login", Since: since(2 * time.Minute)},
		{Branch: "feat/billing", Path: billingPath, Namespace: "acme_feat_billing", Since: since(time.Minute)},
	}, db.Holders)
	assert.Equal(t, runps.Counts{Running: 1, Holders: 3}, shared.Counts)
	assert.True(t, shared.Shared)
}

func TestARepositoryNamedNarrowsTheListToItsJobs(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme(), other()}}, runps.Request{Repo: loginPath})

	assert.Equal(t, []string{"feat/billing", "fix/login", "shared"}, subjects(read))
}

func TestARepositoryNamedByARelativePathIsReadFromWhereTheCallerStands(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme(), other()}}, runps.Request{Repo: "fix-login"}.From(domain.Caller{Cwd: "/acme.trees"}))

	assert.Equal(t, []string{"feat/billing", "fix/login", "shared"}, subjects(read))
}

func TestARepositoryNamedOutsideAnyIsRefused(t *testing.T) {
	_, err := usecase(seed{Projects: []runps.Project{acme()}}).Run(t.Context(), runps.Request{Repo: "/nowhere"}, domain.RunOptions{})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok)
	assert.Equal(t, domain.CodeNotARepository, refused.Code)
}

func TestJobsWhoseRepositoryCannotBeReadAreWarnedAndListedAllTheSame(t *testing.T) {
	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{acme()}}, runps.Request{})

	assert.Equal(t, []string{runps.CodeRepoUnreadable}, codes(read))
	assert.Equal(t, []string{"tool:running"}, names(group(t, read, otherPath).Jobs))
}

func TestADaemonThatCannotTellWhichJobsAreUpIsWarned(t *testing.T) {
	read := read(t, seed{JobsErr: run.ErrUnknown, Projects: []runps.Project{acme()}}, runps.Request{})

	assert.Empty(t, read.Items)
	assert.Equal(t, []string{runps.Name + domain.JobsUnknown}, codes(read))
}

func TestAWorktreeWhoseEveryJobEndedKeepsItsProfile(t *testing.T) {
	ended := slices.DeleteFunc(jobs(), func(job run.Job) bool { return job.WorkDir == billingPath && job.Up })

	billing := group(t, read(t, seed{Jobs: ended, Projects: []runps.Project{acme()}}, runps.Request{}), "feat/billing")

	assert.Equal(t, "dev", billing.Profile)
	assert.Equal(t, []string{"worker:crashed", "migrate:exited", "web:not_started", "api:not_started"}, names(billing.Jobs))
}

func TestAWorktreeWhoseRecordDoesNotReadIsWarnedOnItsGroup(t *testing.T) {
	p := acme()
	p.RecordErrs = map[string]error{"fix/login": errors.New("bad json")}

	read := read(t, seed{Jobs: jobs(), Projects: []runps.Project{p}}, runps.Request{})

	for _, item := range read.Items {
		if item.Subject == "fix/login" {
			require.Len(t, item.Warnings, 1)
			assert.Equal(t, runps.CodeMetaUnreadable, item.Warnings[0].Code)
		}
	}
}
