package runjobs_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveNeverStartsTheDaemonItsJobsUnknown(t *testing.T) {
	d := &daemon{unknown: true}

	running, err := stops(d).Observe(t.Context(), runjobs.ObserveStopParams{Command: "run-down", Repository: repo})

	require.NoError(t, err)
	assert.True(t, running.Unknown)
	assert.False(t, d.ensured)
}

func TestReachStartsADaemonThatDiedToTakeItsJobsOver(t *testing.T) {
	d := &daemon{unknown: true, listed: []run.Job{{Name: "api", WorkDir: featPath, Up: true, State: run.Running, Hold: run.HoldOwn}}}

	running, err := stops(d).Reach(t.Context(), runjobs.ObserveStopParams{Command: "run-down", Repository: repo})

	require.NoError(t, err)
	assert.False(t, running.Unknown)
	assert.Equal(t, []string{"ensure"}, d.calls)
	assert.Len(t, running.Jobs, 1)
}

func TestReachRefusesADaemonStartedThatStillCannotTell(t *testing.T) {
	d := &daemon{unknown: true, staysUnknown: true}

	_, err := stops(d).Reach(t.Context(), runjobs.ObserveStopParams{Command: "run-stop", Repository: repo})

	refused, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, "run-stop"+runjobs.DaemonUnavailable, refused.Code)
	assert.True(t, d.ensured)
}
