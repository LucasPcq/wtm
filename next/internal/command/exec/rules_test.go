package exec_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllRunsInEveryWorktreeThatStandsInGitsOrderAndWarnsOfTheGoneOnes(t *testing.T) {
	w := newWorld(t)

	planned, warnings := w.plan(t, exec.Request{All: true, Command: "true"})

	assert.Equal(t, []string{"main", w.at("detached"), featA, featB, numbered, unnumbered}, names(planned.Targets))
	require.Contains(t, codes(warnings), exec.CodeWorktreesGone)
	assert.Equal(t, gone, warnings[0].Params["worktrees"])
	assert.Equal(t, "prune", warnings[0].FollowUp.Command)
}

func TestTheTargetsArePathsAsGitSpellsThem(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, running("true", featA, w.at("detached")))

	assert.Equal(t, []exec.Target{{Worktree: featA, Branch: featA, Path: w.at(featA)}, {Worktree: w.at("detached"), Path: w.at("detached")}}, planned.Targets)
}

func TestTheConcurrencyNeverExceedsTheTargets(t *testing.T) {
	w := newWorld(t)
	for name, c := range map[string]struct {
		jobs string
		want int
	}{
		"cpus":   {"", 2},
		"zero":   {"0", 2},
		"one":    {"1", 1},
		"capped": {"6", 2},
	} {
		t.Run(name, func(t *testing.T) {
			planned, _ := w.plan(t, exec.Request{Worktrees: []string{featA, featB}, Command: "true", Jobs: c.jobs})

			assert.Equal(t, c.want, planned.Jobs)
			assert.Equal(t, "Concurrency", planned.Recap[2].Label)
			assert.Equal(t, planned.Jobs, atoi(t, planned.Recap[2].Values[0].Value))
		})
	}
}

func TestATargetWhoseRecordDoesNotReadIsWarnedOfWhyItRunsWithoutItsEnvironment(t *testing.T) {
	cases := map[string]struct {
		Record string
		Cause  string
	}{
		"corrupt": {Record: "{not json", Cause: "invalid character"},
		"newer":   {Record: `{"schema": 99, "ordinal": 3, "isolation": "isolated"}`, Cause: "is at schema 99, newer than the 1 this wtm writes"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.rawRecord(t, featA, c.Record)

			_, warnings := w.plan(t, running("true", featA, numbered))

			require.Equal(t, []string{exec.CodeMetaUnreadable}, codes(warnings))
			assert.Equal(t, featA, warnings[0].Params["branch"])
			assert.Contains(t, warnings[0].Message, c.Cause)
		})
	}
}

func TestAnIsolatedWorktreeWithNoOrdinalIsWarnedOfAndNothingIsAllocated(t *testing.T) {
	w := newWorld(t)

	_, warnings := w.plan(t, running("true", unnumbered, numbered))

	assert.Equal(t, []string{exec.CodeEnvUnnumbered}, codes(warnings))
	assert.Equal(t, unnumbered, warnings[0].Params["worktree"])
	record, err := w.meta.Load(unnumbered)
	require.NoError(t, err)
	assert.Zero(t, record.Ordinal)
}

func TestTheLocksAreSharedOnTheRepositoryAndEachTarget(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, running("true", featB, featA))

	assert.Equal(t, []domain.Lock{
		{Scope: domain.ScopeRepo, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: featA, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: featB, Mode: domain.LockShared},
	}, planned.Locks())
}
