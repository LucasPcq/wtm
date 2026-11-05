package runjobs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARestartStopsTheJobThenStartsItWithinOnePhaseItsLogContinued(t *testing.T) {
	api := run.Job{Name: "api", WorkDir: featPath, Up: true, State: run.Running, Hold: run.HoldOwn, Profile: "dev"}
	d := &daemon{listed: []run.Job{api}}
	facts := runjobs.Facts{Repository: repo, Records: map[string]worktree.Meta{featA: {Ordinal: 1}}, Jobs: []run.Job{api}}
	target := runjobs.Target{Worktree: featA, Branch: featA, Path: featPath, Numbered: true, Jobs: []runjobs.JobPlan{{Name: "api", Kind: project.Service, Action: runjobs.ActionRestart, Step: 1}}}
	var events []domain.Event

	started := starts(d).Start(t.Context(), runjobs.StartParams{Command: "run-start", Facts: facts, Target: target, Restart: true, NoProbe: true, Report: func(e domain.Event) { events = append(events, e) }})

	assert.Equal(t, domain.StatusDone, started.Status, started.Err)
	assert.Equal(t, []string{"stop api", "start api"}, d.calls)
	require.Len(t, d.started, 1)
	assert.True(t, d.started[0].Restart, "the log continues behind the run mark")
	assert.Equal(t, "dev", d.started[0].Profile, "the profile it was started under is kept")
	assert.Equal(t, []runjobs.JobResult{{Name: "api", Kind: project.Service, Status: runjobs.JobStarted, Log: started.Jobs[0].Log}}, started.Jobs)
	phases := 0
	for _, event := range events {
		if _, ok := event.(domain.PhaseStarted); ok {
			phases++
		}
	}
	assert.Equal(t, 1, phases, "the stop and the start are one phase")
}
