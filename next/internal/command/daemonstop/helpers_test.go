package daemonstop_test

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

// usecase is daemon stop over d, under ops.
func usecase(d *daemon, ops operation.Service) daemonstop.UseCase {
	return daemonstop.UseCase{Operations: ops, Daemon: d}
}

// operations is an operations service over in-memory locks and events.
func operations() operation.Service {
	locks := &operationtest.Locks{}
	return operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now}
}
