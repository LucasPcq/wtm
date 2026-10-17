package oplock

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

var startedAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

var (
	repoShared = wtm.Lock{Scope: wtm.ScopeRepo, Mode: wtm.LockShared}
	featA      = wtm.Lock{Scope: wtm.ScopeWorktree, Name: "feat/a", Mode: wtm.LockExclusive}
	featB      = wtm.Lock{Scope: wtm.ScopeWorktree, Name: "feat/b", Mode: wtm.LockExclusive}
)

func op(id string, locks ...wtm.Lock) wtm.Operation {
	return wtm.Operation{ID: id, PID: 4242, Command: "create", Surface: "tui", StartedAt: startedAt, Locks: locks}
}
