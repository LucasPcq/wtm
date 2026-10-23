package provision_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASourceWithoutWorktreeIsMovedWithoutCheckingItOut(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{})
	before := gittest.Output(t, u.main, "rev-parse", existing)

	forwarded := u.service.FastForward(t.Context(), provision.FastForwardParams{Subject: existing})

	assert.Equal(t, provision.FastForwarded{}, forwarded)
	assert.Equal(t, gittest.Output(t, u.main, "rev-parse", "origin/"+existing), gittest.Output(t, u.main, "rev-parse", existing))
	assert.NotEqual(t, before, gittest.Output(t, u.main, "rev-parse", existing))
}

func TestASourceCheckedOutIsMovedInItsWorktree(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{Linked: true})
	before := gittest.Output(t, u.linked, "rev-parse", "HEAD")

	forwarded := u.service.FastForward(t.Context(), provision.FastForwardParams{Subject: existing, CheckedOutAt: u.linked})

	assert.Equal(t, provision.FastForwarded{}, forwarded)
	assert.Equal(t, gittest.Output(t, u.main, "rev-parse", "origin/"+existing), gittest.Output(t, u.linked, "rev-parse", "HEAD"))
	assert.NotEqual(t, before, gittest.Output(t, u.linked, "rev-parse", "HEAD"))
}

func TestASourceWithUncommittedChangesIsLeftWhereItIs(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{Linked: true, Dirty: true})
	before := gittest.Output(t, u.linked, "rev-parse", "HEAD")

	forwarded := u.service.FastForward(t.Context(), provision.FastForwardParams{Subject: existing, CheckedOutAt: u.linked})

	assert.Equal(t, provision.FastForwarded{Dirty: true}, forwarded)
	assert.Equal(t, before, gittest.Output(t, u.linked, "rev-parse", "HEAD"))
}

func TestASourceWorktreeStoppedInARebaseIsLeftWhereItIs(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{Linked: true, Rebasing: true})
	head, branch := gittest.Output(t, u.linked, "rev-parse", "HEAD"), gittest.Output(t, u.main, "rev-parse", existing)

	forwarded := u.service.FastForward(t.Context(), provision.FastForwardParams{Subject: existing, CheckedOutAt: u.linked})

	assert.Equal(t, provision.FastForwarded{Busy: true}, forwarded)
	assert.Equal(t, head, gittest.Output(t, u.linked, "rev-parse", "HEAD"))
	assert.Equal(t, branch, gittest.Output(t, u.main, "rev-parse", existing))
}

func TestAFastForwardThatFailsCarriesItsCause(t *testing.T) {
	for name, seed := range map[string]upstreamSeed{"without_worktree": {}, "in_worktree": {Linked: true}} {
		t.Run(name, func(t *testing.T) {
			u := behindOrigin(t, seed)
			before := gittest.Output(t, u.main, "rev-parse", existing)
			gittest.Git(t, u.main, "remote", "set-url", "origin", u.main+"/gone")

			forwarded := u.service.FastForward(t.Context(), provision.FastForwardParams{Subject: existing, CheckedOutAt: u.linked})

			require.Error(t, forwarded.Err)
			assert.False(t, forwarded.Dirty)
			assert.Equal(t, before, gittest.Output(t, u.main, "rev-parse", existing))
		})
	}
}

func TestEachSubjectIsFastForwardedBetweenItsPhaseEventsAndThoseLeftBehindAreWarnedOf(t *testing.T) {
	cases := map[string]struct {
		seed     upstreamSeed
		advanced []string
		warnings []domain.Warning
	}{
		"moved": {upstreamSeed{}, []string{existing}, nil},
		"left_dirty": {upstreamSeed{Linked: true, Dirty: true}, nil, []domain.Warning{{
			Code:    "create.ff_dirty",
			Message: "couldn't fast-forward " + existing + ": its worktree has uncommitted changes — it starts from your local branch",
			Params:  domain.Params{"branch": existing},
		}}},
		"left_busy": {upstreamSeed{Linked: true, Rebasing: true}, nil, []domain.Warning{{
			Code:    "create.ff_busy",
			Message: "couldn't fast-forward " + existing + ": its worktree is in the middle of a rebase or a bisect — it starts from your local branch",
			Params:  domain.Params{"branch": existing},
		}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			u := behindOrigin(t, c.seed)
			report := &recorder{}

			advanced, warnings := u.service.FastForwardEach(t.Context(), fastForwardEach(u.snapshotOf(), existing), report.add)

			assert.Equal(t, c.advanced, advanced)
			assert.Equal(t, c.warnings, warnings)
			assert.Equal(t, []domain.Event{
				domain.PhaseStarted{Subject: existing, Phase: "fast_forward"},
				domain.PhaseFinished{Subject: existing, Phase: "fast_forward", Failed: c.warnings != nil},
			}, report.all())
		})
	}
}

func TestASubjectThatFailsToFastForwardIsWarnedOfWithItsCause(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{})
	gittest.Git(t, u.main, "remote", "set-url", "origin", u.main+"/gone")

	advanced, warnings := u.service.FastForwardEach(t.Context(), fastForwardEach(u.snapshotOf(), existing), (&recorder{}).add)

	assert.Empty(t, advanced)
	require.Len(t, warnings, 1)
	assert.Equal(t, "create.ff_failed", warnings[0].Code)
	assert.NotEmpty(t, warnings[0].Params["cause"])
	assert.Equal(t, "couldn't fast-forward "+existing+": "+warnings[0].Params["cause"]+" — it starts from your local branch", warnings[0].Message)
}

func TestNoSubjectIsFastForwardedOnceCancelled(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{})
	before := gittest.Output(t, u.main, "rev-parse", existing)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report := &recorder{}

	advanced, warnings := u.service.FastForwardEach(ctx, fastForwardEach(worktree.Snapshot{}, existing), report.add)

	assert.Empty(t, advanced)
	assert.Empty(t, warnings)
	assert.Empty(t, report.all())
	assert.Equal(t, before, gittest.Output(t, u.main, "rev-parse", existing))
}

func TestEachSubjectIsFastForwardedInTurnAFailureStoppingNone(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{})

	advanced, warnings := u.service.FastForwardEach(t.Context(), fastForwardEach(worktree.Snapshot{}, "gone", existing), (&recorder{}).add)

	assert.Equal(t, []string{existing}, advanced)
	assert.Equal(t, []string{"create.ff_failed"}, codes(warnings))
	assert.Equal(t, "gone", warnings[0].Params["branch"])
}

func TestASubjectNotStartedOnceCancelledIsNeitherAdvancedNorWarnedOf(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{})
	ctx, cancel := context.WithCancel(t.Context())
	report := &recorder{on: func(event domain.Event) {
		if _, finished := event.(domain.PhaseFinished); finished {
			cancel()
		}
	}}

	advanced, warnings := u.service.FastForwardEach(ctx, fastForwardEach(worktree.Snapshot{}, existing, "later"), report.add)

	assert.Equal(t, []string{existing}, advanced)
	assert.Empty(t, warnings)
	assert.Len(t, report.all(), 2)
}
