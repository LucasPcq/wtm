package daemonstop_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStopSaysWhatStopsWhatRunsOnAndWhatItsHoldersLetGo(t *testing.T) {
	plan := daemonstop.Decide(running(web, db, dbHeld, cache, cacheHeld).status)

	assert.True(t, plan.Confirm)
	assert.Equal(t, []string{"web (main · acme)", "db (main · acme)"}, run.Names(plan.Stops))
	assert.Equal(t, []string{"cache (main · acme)", "cache (feat/a · acme)"}, run.Names(plan.Kept))
	assert.Equal(t, []string{"db (feat/a · acme)"}, run.Names(plan.Lost))
}

// Legacy's restart said "Kept: db (feat-a)" of a claim on a shared service the shutdown stopped: the next daemon dropped it.
func TestAClaimOnAServiceTheShutdownStopsIsNoClaimKept(t *testing.T) {
	plan := daemonstop.Decide(running(db, dbHeld).status)

	assert.Empty(t, plan.Kept)
	assert.Equal(t, []string{"db (feat/a · acme)"}, run.Names(plan.Lost))
}

func TestAStopCuttingNoServiceNeedsNoConfirmation(t *testing.T) {
	cases := map[string]*daemon{
		"stacks_only": running(cache, cacheHeld),
		"none":        running(),
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			assert.False(t, daemonstop.Decide(d.status).Confirm)
		})
	}
}

func TestADaemonWhoseJobsCannotBeReadIsConfirmedBeforeItStops(t *testing.T) {
	d := running()
	d.status.Skew = run.SkewProtocol

	plan := daemonstop.Decide(d.status)

	assert.True(t, plan.JobsUnreadable)
	assert.True(t, plan.Confirm)
}

func TestTheDaemonStopsOnceAsked(t *testing.T) {
	d := running(web, cache)

	outcome, err := usecase(d, operations()).Run(t.Context(), daemonstop.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, 1, d.shutdowns)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
}

func TestWithNoDaemonRunningNothingIsStopped(t *testing.T) {
	d := &daemon{}

	outcome, err := usecase(d, operations()).Run(t.Context(), daemonstop.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, outcome.Plan.Empty)
	assert.Empty(t, outcome.Result.Items)
	assert.Zero(t, d.shutdowns)
}

func TestADaemonThatDoesNotStopFailsItsItem(t *testing.T) {
	d := running(web)
	d.fail = errStuck

	outcome, err := usecase(d, operations()).Run(t.Context(), daemonstop.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, outcome.Result.Items[0].Status)
	assert.Equal(t, daemonstop.CodeShutdownFailed, outcome.Result.Items[0].Err.Code)
}
