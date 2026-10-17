package oplock

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
)

var startedAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

var (
	repoShared = operation.Lock{Scope: operation.ScopeRepo, Mode: operation.Shared}
	featA      = operation.Lock{Scope: operation.ScopeWorktree, Name: "feat/a", Mode: operation.Exclusive}
	featB      = operation.Lock{Scope: operation.ScopeWorktree, Name: "feat/b", Mode: operation.Exclusive}
)

func op(id string, locks ...operation.Lock) operation.Operation {
	return operation.Operation{ID: id, PID: 4242, Command: "create", Surface: "tui", StartedAt: startedAt, Locks: locks}
}
