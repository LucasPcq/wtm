package daemonstatus_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/require"
)

// restart is the follow-up a skewed daemon's warning offers.
var restart = &domain.FollowUp{Command: domain.CommandDaemonRestart}

// read runs daemon status over a daemon that reads as daemon.
func read(t *testing.T, daemon run.Daemon) daemonstatus.Outcome {
	t.Helper()
	usecase := daemonstatus.UseCase{Daemon: func(context.Context) (run.Daemon, error) { return daemon, nil }, Version: build, Restart: restart}
	outcome, err := usecase.Run(t.Context(), daemonstatus.Request{}, domain.RunOptions{})
	require.NoError(t, err)
	require.Len(t, outcome.Items, 1)
	return outcome
}

// codes are the codes of warnings.
func codes(warnings []domain.Warning) []string {
	var codes []string
	for _, w := range warnings {
		codes = append(codes, w.Code)
	}
	return codes
}
