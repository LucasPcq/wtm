package provision_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
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
	for name, seed := range map[string]upstreamSeed{"without worktree": {}, "in its worktree": {Linked: true}} {
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

func TestEachSubjectIsFastForwardedBetweenItsPhaseEventsAndOnlyThoseLeftBehindAreReturned(t *testing.T) {
	cases := map[string]struct {
		seed   upstreamSeed
		missed []provision.Missed
	}{
		"moved":      {upstreamSeed{}, nil},
		"left dirty": {upstreamSeed{Linked: true, Dirty: true}, []provision.Missed{{Subject: existing, FastForwarded: provision.FastForwarded{Dirty: true}}}},
		"left busy":  {upstreamSeed{Linked: true, Rebasing: true}, []provision.Missed{{Subject: existing, FastForwarded: provision.FastForwarded{Busy: true}}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			u := behindOrigin(t, c.seed)
			report := &recorder{}

			missed := u.service.FastForwardEach(t.Context(), provision.FastForwardEachParams{Subjects: []string{existing}, Snapshot: u.snapshotOf(), Report: report.add})

			assert.Equal(t, c.missed, missed)
			assert.Equal(t, []domain.Event{
				domain.PhaseStarted{Subject: existing, Phase: "fast_forward"},
				domain.PhaseFinished{Subject: existing, Phase: "fast_forward", Failed: c.missed != nil},
			}, report.all())
		})
	}
}

func TestNoSubjectIsFastForwardedOnceCancelled(t *testing.T) {
	u := behindOrigin(t, upstreamSeed{})
	before := gittest.Output(t, u.main, "rev-parse", existing)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report := &recorder{}

	missed := u.service.FastForwardEach(ctx, provision.FastForwardEachParams{Subjects: []string{existing}, Report: report.add})

	assert.Empty(t, missed)
	assert.Empty(t, report.all())
	assert.Equal(t, before, gittest.Output(t, u.main, "rev-parse", existing))
}
