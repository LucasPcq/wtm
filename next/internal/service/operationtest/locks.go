package operationtest

import (
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
)

// Locks is the Locker and the Publisher of an operations service, in memory.
// Take records the operation and hands it a hold that is protected, never
// grows and is released, or fails with Err once recorded; Open records the
// operation and publishes its events to Published. It is safe for concurrent
// use.
type Locks struct {
	Err       error
	Published Recorder

	mu     sync.Mutex
	taken  []domain.Operation
	opened []domain.Operation
	holds  []*hold
}

// Take records op and returns its hold, or Err when set.
func (l *Locks) Take(op domain.Operation) (operation.Hold, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.taken = append(l.taken, op)
	if l.Err != nil {
		return nil, l.Err
	}
	h := &hold{locks: l}
	l.holds = append(l.holds, h)
	return h, nil
}

// Open records op and publishes every event of it through the one report it
// returns: the record op carries is what stamps each of them.
func (l *Locks) Open(op domain.Operation) domain.Report {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.opened = append(l.opened, op)
	return l.Published.Add
}

// Taken returns the operations Take was called with, in order.
func (l *Locks) Taken() []domain.Operation {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.taken)
}

// Opened returns the operations Open was called with, in order.
func (l *Locks) Opened() []domain.Operation {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.opened)
}

// Released reports whether every hold Take handed out was released.
func (l *Locks) Released() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !slices.ContainsFunc(l.holds, func(h *hold) bool { return !h.released })
}

func (l *Locks) held() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.holds)
}

// hold never grows: the contract does not check planning again under new locks, PlanUnderLocks's own tests do.
type hold struct {
	locks    *Locks
	released bool
}

func (h *hold) Grow([]domain.Lock) (bool, error) { return false, nil }

func (h *hold) Protected() bool { return true }

func (h *hold) Release() {
	h.locks.mu.Lock()
	defer h.locks.mu.Unlock()
	h.released = true
}
