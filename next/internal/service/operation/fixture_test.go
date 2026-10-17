package operation

import (
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

var (
	repoLock    = domain.Lock{Scope: domain.ScopeRepo, Mode: domain.LockShared}
	featureLock = domain.Lock{Scope: domain.ScopeWorktree, Name: "feat/a", Mode: domain.LockExclusive}
	startedAt   = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
)

type locker struct {
	taken []domain.Operation
	hold  *hold
	err   error
}

func (l *locker) Take(op domain.Operation) (Hold, error) {
	l.taken = append(l.taken, op)
	if l.err != nil {
		return nil, l.err
	}
	return l.hold, nil
}

type hold struct {
	held        []domain.Lock
	unprotected bool
	released    bool
}

func (h *hold) Grow(locks []domain.Lock) (bool, error) {
	grown := false
	for _, lock := range locks {
		if !slices.Contains(h.held, lock) {
			h.held = append(h.held, lock)
			grown = true
		}
	}
	return grown, nil
}

func (h *hold) Protected() bool { return !h.unprotected }

func (h *hold) Release() { h.released = true }

type publisher struct {
	opened    []domain.Operation
	published []domain.Event
}

func (p *publisher) Open(op domain.Operation) domain.Report {
	p.opened = append(p.opened, op)
	return func(event domain.Event) { p.published = append(p.published, event) }
}

type plan struct{ locks []domain.Lock }

func (p plan) Locks() []domain.Lock { return p.locks }
