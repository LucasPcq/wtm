package runlist_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWorktreeTheCallerStandsInIsListedWithItsProfilesAndJobsInRunTomlsOrder(t *testing.T) {
	d := read(t, seed{Run: declared}, runlist.Request{})

	assert.Equal(t, "feat/a", d.Branch)
	assert.Equal(t, []string{"dev", "full"}, []string{d.Profiles[0].Name, d.Profiles[1].Name})
	assert.True(t, d.Profiles[0].Default)
	var names []string
	for _, j := range d.Jobs {
		names = append(names, j.Name)
	}
	assert.Equal(t, []string{"web", "api", "migrate", "db", "cache", "stack"}, names)
}

func TestAJobsPortsAreTheWorktreesNotRunTomlsBase(t *testing.T) {
	d := read(t, seed{Run: declared}, runlist.Request{})

	assert.Equal(t, map[string]int{"PORT": 3010}, job(t, d, "web").Ports, "feat/a's ordinal 1 puts its ports 10 above main's")
	assert.Equal(t, map[string]int{"DB_PORT": 5432}, job(t, d, "db").Ports, "a shared job binds its base in every worktree")
	assert.Equal(t, map[string]int{"PORT": 3000}, job(t, read(t, seed{Run: declared}, runlist.Request{Worktree: "main"}), "web").Ports)
}

func TestEachJobSaysWhereItStandsInTheWorktree(t *testing.T) {
	exit1 := 1
	s := seed{Run: declared, Logged: []string{"migrate"}, Jobs: []run.Job{
		{Name: "web", WorkDir: featA, Profile: "dev", Up: true, State: run.Running, Hold: run.HoldOwn, URL: "http://web.feat-a.localhost"},
		{Name: "api", WorkDir: featA, Profile: "dev", State: run.Crashed, Hold: run.HoldOwn, ExitCode: &exit1},
		{Name: "db", WorkDir: featA, SharedDir: mainPath, Up: true, Hold: run.HoldClaim},
		{Name: "web", WorkDir: mainPath, Up: true, State: run.Running, Hold: run.HoldOwn},
		{Name: "stack", WorkDir: featA, Up: true, State: run.Running, Hold: run.HoldDetached},
	}}

	d := read(t, s, runlist.Request{})

	assert.Equal(t, run.Running, stateOf(t, d, "web"))
	assert.Equal(t, "http://web.feat-a.localhost", job(t, d, "web").URL)
	assert.Equal(t, run.Crashed, stateOf(t, d, "api"))
	assert.Equal(t, run.Running, stateOf(t, d, "db"), "a claim reads as the shared job it holds")
	assert.Equal(t, run.HoldClaim, job(t, d, "db").Hold)
	assert.Equal(t, run.Stopped, stateOf(t, d, "migrate"), "a job its log says ran, the daemon no longer listing it")
	assert.Equal(t, run.Running, stateOf(t, d, "cache"), "a job its runner starts reads as its runner")
	assert.Equal(t, run.NotStarted, stateOf(t, read(t, s, runlist.Request{Worktree: "main"}), "stack"), "a job that never ran")
	assert.Equal(t, "dev", d.Profile)
	assert.True(t, d.Profiles[0].Running)
}

func TestADaemonThatCannotTellLeavesEachStateUnknownAndIsWarned(t *testing.T) {
	outcome, err := usecase(seed{Run: declared, JobsErr: run.ErrUnknown}).Run(t.Context(), runlist.Request{}.From(domain.Caller{Cwd: featA}), domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, outcome.Items[0].Detail.Jobs[0].State)
	require.Len(t, outcome.Warnings, 1)
	assert.Equal(t, runlist.Name+domain.JobsUnknown, outcome.Warnings[0].Code)
}

func TestAProfileOffersToStartItAndStopItsJobsAJobToStartStopAndReadIt(t *testing.T) {
	d := read(t, seed{Run: declared}, runlist.Request{})

	assert.Equal(t, [][]string{
		{domain.CommandRunUp, "worktree=feat/a", "profile=dev"},
		{domain.CommandRunDown, "worktree=feat/a", "profile=dev"},
	}, commands(d.Profiles[0].Actions))
	assert.Equal(t, [][]string{
		{domain.CommandRunStart, "worktree=feat/a", "job=web"},
		{domain.CommandRunStop, "worktree=feat/a", "job=web"},
		{domain.CommandRunLogs, "worktree=feat/a", "job=web"},
	}, commands(d.Jobs[0].Actions))
}

func TestARepositoryWithNoRunTomlIsFollowedUpByRunInit(t *testing.T) {
	_, err := usecase(seed{Run: project.Run{}}).Run(t.Context(), runlist.Request{}, domain.RunOptions{})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok)
	assert.Equal(t, runlist.CodeNotInitialized, refused.Code)
	require.NotNil(t, refused.FollowUp)
}

func TestAnInvalidRunTomlIsRefused(t *testing.T) {
	_, err := usecase(seed{RunErr: errors.New("line 3: bad key")}).Run(t.Context(), runlist.Request{}, domain.RunOptions{})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok)
	assert.Equal(t, runlist.CodeRunInvalid, refused.Code)
}

func TestAWorktreeNamedThatDoesNotExistIsAProblemOfItsField(t *testing.T) {
	_, err := usecase(seed{Run: declared}).Run(t.Context(), runlist.Request{Worktree: "feat/nope"}, domain.RunOptions{})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok)
	require.Len(t, refused.Fields, 1)
	assert.Equal(t, domain.PathWorktree, refused.Fields[0].Path)
}

func TestAWorktreeWhoseRecordDoesNotReadIsWarnedItsPortsUnknown(t *testing.T) {
	outcome, err := usecase(seed{Run: declared, Unread: true}).Run(t.Context(), runlist.Request{}.From(domain.Caller{Cwd: featA}), domain.RunOptions{})

	require.NoError(t, err)
	item := outcome.Items[0]
	require.Len(t, item.Warnings, 1)
	assert.Equal(t, runlist.CodeMetaUnreadable, item.Warnings[0].Code)
	assert.Nil(t, item.Detail.Jobs[0].Ports)
}
