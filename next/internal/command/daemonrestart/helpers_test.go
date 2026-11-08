package daemonrestart_test

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"

	"github.com/stretchr/testify/require"
)

const build = "0.31.0"

// usecase is daemon restart over d, under ops.
func usecase(d *daemon, ops operation.Service) daemonrestart.UseCase {
	return daemonrestart.UseCase{Operations: ops, Daemon: d, Version: build}
}

// operations is an operations service over in-memory locks and events.
func operations() operation.Service {
	locks := &operationtest.Locks{}
	return operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now}
}

// restarted runs a restart of d to its end, unattended, and returns its one item.
func restarted(t *testing.T, d *daemon, opts domain.RunOptions) (daemonrestart.Outcome, daemonrestart.Item) {
	t.Helper()
	outcome, err := usecase(d, operations()).Run(t.Context(), daemonrestart.Request{}, opts)
	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	require.Len(t, outcome.Result.Items, 1)
	return outcome, outcome.Result.Items[0]
}
