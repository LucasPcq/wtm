package oplock

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// startedAt is when the operations of a test started.
var startedAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// The locks a test's operations take.
var (
	repoShared = domain.Lock{Scope: domain.ScopeRepo, Mode: domain.LockShared}
	featA      = domain.Lock{Scope: domain.ScopeWorktree, Name: "feat/a", Mode: domain.LockExclusive}
	featB      = domain.Lock{Scope: domain.ScopeWorktree, Name: "feat/b", Mode: domain.LockExclusive}
)

// op returns a create operation id taking locks.
func op(id string, locks ...domain.Lock) domain.Operation {
	return domain.Operation{ID: id, PID: 4242, Command: "create", Surface: "tui", StartedAt: startedAt, Locks: locks}
}
