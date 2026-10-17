package oplock

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

var startedAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

var (
	repoShared = domain.Lock{Scope: domain.ScopeRepo, Mode: domain.LockShared}
	featA      = domain.Lock{Scope: domain.ScopeWorktree, Name: "feat/a", Mode: domain.LockExclusive}
	featB      = domain.Lock{Scope: domain.ScopeWorktree, Name: "feat/b", Mode: domain.LockExclusive}
)

func op(id string, locks ...domain.Lock) domain.Operation {
	return domain.Operation{ID: id, PID: 4242, Command: "create", Surface: "tui", StartedAt: startedAt, Locks: locks}
}
