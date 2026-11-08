package daemonrestart_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N4: legacy's restart, and next's before, brought no service back.
func TestARestartBringsTheServicesBackTheStacksTakenOverATaskNotRunAgain(t *testing.T) {
	plan, _ := daemonrestart.Decide(daemonrestart.Facts{Daemon: running("0.30.9", web, db, dbHeld, migrate, stack, api).status, Records: running("0.30.9", web, db, dbHeld, migrate, stack, api).records, Version: build})

	assert.Equal(t, []string{"db (main · acme)", "web (main · acme)", "api (feat/a · acme)", "db (feat/a · acme)"}, run.Names(plan.Back))
	assert.Equal(t, []string{"mail (main · acme)"}, run.Names(plan.TakenOver))
	assert.Equal(t, []string{"migrate (main · acme)"}, run.Names(plan.Tasks))
	assert.Empty(t, plan.Unrecorded)
	assert.True(t, plan.Confirm)
}

func TestTheServicesComeBackFromTheirRecordsTheClaimsLast(t *testing.T) {
	d := running("0.30.9", web, db, dbHeld, stack, api)

	_, item := restarted(t, d, domain.RunOptions{CorrelationID: "restart-1"})

	assert.Equal(t, domain.StatusDone, item.Status, item.Err)
	assert.Equal(t, []string{"shutdown", "ensure", "start web " + main, "start api " + featA, "start db " + featA}, d.calls)
	for _, spec := range d.started {
		assert.Equal(t, "dev", spec.Profile)
		assert.Equal(t, "3000", spec.Env["PORT"])
		assert.Equal(t, "restart-1", spec.Origin.CorrelationID)
	}
}

// N1: next's restart said "Kept: db (feat-a)" of a claim the new daemon dropped at once.
func TestAClaimOnASharedServiceComesBackWithIt(t *testing.T) {
	d := running("0.30.9", db, dbHeld)

	outcome, item := restarted(t, d, domain.RunOptions{})

	assert.Empty(t, outcome.Plan.TakenOver)
	assert.Equal(t, []string{"shutdown", "ensure", "start db " + featA}, d.calls)
	assert.Equal(t, []daemonrestart.Back{{Name: "db", Path: featA, Back: true}}, item.Detail.Back)
}

func TestASharedServiceItsMainCheckoutHoldsComesBackOnItsOwn(t *testing.T) {
	held := db
	held.MainHolds = true
	d := running("0.30.9", held)

	restarted(t, d, domain.RunOptions{})

	assert.Equal(t, []string{"shutdown", "ensure", "start db " + main}, d.calls)
}

func TestAJobThatDoesNotComeBackFailsTheRestartTheOthersBack(t *testing.T) {
	d := running("0.30.9", web, api)
	d.refuse = map[string]error{"web " + main: errPort}

	_, item := restarted(t, d, domain.RunOptions{})

	assert.Equal(t, domain.StatusFailed, item.Status)
	require.NotNil(t, item.Err)
	assert.Equal(t, daemonrestart.CodeNotBack, item.Err.Code)
	assert.Contains(t, item.Err.Message, "web (/acme): "+errPort.Error())
	assert.Equal(t, []daemonrestart.Back{{Name: "web", Path: main, Message: errPort.Error()}, {Name: "api", Path: featA, Back: true}}, item.Detail.Back)
}

func TestAJobTheNewDaemonFindsUpIsBack(t *testing.T) {
	d := running("0.30.9", web)
	d.refuse = map[string]error{"web " + main: &run.Error{Code: run.CodeJobRunning, Message: "job web is already running"}}

	_, item := restarted(t, d, domain.RunOptions{})

	assert.Equal(t, domain.StatusDone, item.Status, item.Err)
}

func TestANewDaemonThatDoesNotStartFailsTheRestart(t *testing.T) {
	d := running("0.30.9", web)
	d.ensure = &run.Error{Code: run.CodeStartFailed, Message: "wtm's daemon did not start within 5s"}

	_, item := restarted(t, d, domain.RunOptions{})

	assert.Equal(t, domain.StatusFailed, item.Status)
	assert.Equal(t, daemonrestart.CodeStartFailed, item.Err.Code)
	assert.Equal(t, []string{"shutdown", "ensure"}, d.calls)
}

func TestWithNoDaemonRunningOneStarts(t *testing.T) {
	d := &daemon{status: run.Daemon{Jobs: []run.Job{stack}}}

	outcome, item := restarted(t, d, domain.RunOptions{})

	assert.False(t, outcome.Plan.Running)
	assert.False(t, outcome.Plan.Confirm)
	assert.Equal(t, domain.StatusDone, item.Status)
	assert.Equal(t, []string{"ensure"}, d.calls)
	assert.Equal(t, "none runs: one starts, as "+build, outcome.Plan.Recap[0].Values[0].Text)
}

func TestWithAFrozenIndexNothingComesBackAndItIsSaid(t *testing.T) {
	d := running("0.30.9", web, stack)
	d.records, d.status.IndexFrozen = nil, true

	plan, warnings := daemonrestart.Decide(daemonrestart.Facts{Daemon: d.status, Records: d.records, Version: build})

	assert.Equal(t, []string{"mail (main · acme)", "web (main · acme)"}, run.Names(plan.Unrecorded))
	assert.Empty(t, plan.Back)
	assert.Empty(t, plan.TakenOver, "the new daemon reads no index a newer wtm wrote")
	require.Len(t, warnings, 1)
	assert.Equal(t, daemonrestart.CodeIndexFrozen, warnings[0].Code)
}

func TestADaemonOfAnotherProtocolHasItsJobsReadFromItsIndex(t *testing.T) {
	d := running("0.40.0", web, stack)
	d.status.Skew, d.status.Jobs = run.SkewProtocol, nil

	plan, _ := daemonrestart.Decide(daemonrestart.Facts{Daemon: d.status, Records: d.records, Version: build})

	assert.Equal(t, []string{"web (main · acme)"}, run.Names(plan.Back))
	assert.Equal(t, []string{"mail (main · acme)"}, run.Names(plan.TakenOver))
}

// The index keeps no spec.Shared: a claim started again from its record alone was refused, "job db is shared, and no main checkout was named to run it in".
func TestASharedJobComesBackRunningWhereItsOwnRunRan(t *testing.T) {
	d := running("0.30.9", db, dbHeld)
	d.records[0].Spec.Env, d.records[0].Spec.LogDir = map[string]string{"DB_PORT": "5432"}, "/acme/.git/wtm/logs/main"

	restarted(t, d, domain.RunOptions{})

	require.Len(t, d.started, 1)
	assert.Equal(t, &run.Shared{WorkDir: main, Env: map[string]string{"DB_PORT": "5432"}, LogDir: "/acme/.git/wtm/logs/main"}, d.started[0].Shared)
}

func TestAClaimWhoseSharedJobHasNoRecordDoesNotComeBack(t *testing.T) {
	d := running("0.30.9", db, dbHeld)
	d.records = d.records[1:]

	plan, _ := daemonrestart.Decide(daemonrestart.Facts{Daemon: d.status, Records: d.records, Version: build})

	assert.Equal(t, []string{"db (main · acme)", "db (feat/a · acme)"}, run.Names(plan.Unrecorded))
	assert.Empty(t, plan.Back)
}

func TestAClaimTheLegacyWtmRecordedWithNoOriginComesBackForItsWorktree(t *testing.T) {
	d := running("0.30.9", db, dbHeld, api)
	d.records[1].Spec.Origin = nil

	restarted(t, d, domain.RunOptions{CorrelationID: "restart-1"})

	claim := d.started[len(d.started)-1]
	require.Equal(t, featA, claim.WorkDir)
	assert.Equal(t, &run.Origin{Root: main, Branch: "feat/a", CorrelationID: "restart-1"}, claim.Origin)
}

func TestARestartInterruptedOnceItsDaemonStoppedStillBringsTheJobsBack(t *testing.T) {
	d := running("0.30.9", web)
	ctx, cancel := context.WithCancel(t.Context())
	d.stopped = cancel

	outcome, err := usecase(d, operations()).Run(ctx, daemonrestart.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)
	assert.Equal(t, []string{"shutdown", "ensure", "start web " + main}, d.calls)
}
