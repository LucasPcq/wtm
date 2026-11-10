package domain

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASkewedDaemonTheClientCannotGoOnWithIsRefusedFollowedByItsRestart(t *testing.T) {
	restart := &FollowUp{Command: "daemon restart"}
	cases := map[string]struct {
		err  error
		skew run.Skew
	}{
		"protocol": {err: &run.Error{Code: run.CodeSkew, Message: "wtm's daemon speaks protocol 1, this wtm 2"}, skew: run.SkewBuild},
		"legacy":   {err: &run.Error{Code: run.CodeLegacy, Message: "wtm's daemon is the legacy wtm"}, skew: run.SkewLegacy},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			refusal := SkewRefusal(SkewRefusalParams{Command: "run-down", Undone: "nothing stopped", Err: c.err, Restart: restart})

			require.NotNil(t, refusal)
			assert.Equal(t, KindPrecondition, refusal.Kind)
			assert.Equal(t, "run-down.daemon_skew", refusal.Code)
			assert.Equal(t, string(c.skew), refusal.Params["skew"])
			require.NotNil(t, refusal.FollowUp)
			assert.Equal(t, "run-down.restart_daemon", refusal.FollowUp.Code)
		})
	}
	answered := SkewRefusal(SkewRefusalParams{Command: "run-up", Undone: "nothing started", Err: &run.Skewed{Skew: run.SkewBuild, Daemon: "0.30.0"}})
	require.NotNil(t, answered)
	assert.Equal(t, "0.30.0", answered.Params["daemon"])
	assert.Nil(t, SkewRefusal(SkewRefusalParams{Command: "run-down", Err: errors.New("boom")}))
}
