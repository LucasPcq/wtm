package provision_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
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
