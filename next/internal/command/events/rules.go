package events

import (
	"cmp"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The facts

// Facts are what a snapshot reads of a repository: its worktrees, and the meta record of each worktree on a branch that has one, by branch.
type Facts struct {
	worktree.Repository
	Metas map[string]worktree.Meta
}

// Identities are the worktrees of facts as the stream names them, those on no branch left out: a reader keys a worktree by its branch.
// A worktree's ordinal is its record's while no other worktree whose branch sorts first claims the same, the main worktree holding 0; its isolation is its record's, isolated without one.
func Identities(facts Facts) []worktree.Identity {
	identities := make([]worktree.Identity, 0, len(facts.Worktrees))
	for _, w := range facts.Worktrees {
		branch := w.Branch()
		if branch == "" {
			continue
		}

		meta := facts.Metas[branch]
		identities = append(identities, worktree.Identity{
			Branch:    branch,
			Path:      w.Path,
			Parent:    meta.SourceBranch,
			Ordinal:   facts.ordinal(w),
			Isolation: meta.Isolation.Effective(),
			Main:      w.Main,
			CreatedAt: meta.CreatedAt,
		})
	}

	return identities
}

// ordinal is the number w holds: 0 for the main worktree, its record's when no other worktree whose branch sorts first claims it, else none yet.
func (f Facts) ordinal(w worktree.Worktree) *int {
	if w.Main {
		return new(0)
	}

	claimed := f.Metas[w.Branch()].Ordinal
	if claimed <= 0 {
		return nil
	}
	for _, other := range f.Worktrees {
		branch := other.Branch()
		if branch == "" || branch == w.Branch() {
			continue
		}
		if f.claim(other) == claimed && branch < w.Branch() {
			return nil
		}
	}

	return new(claimed)
}

// claim is the ordinal w claims: 0 for the main worktree, else its record's.
func (f Facts) claim(w worktree.Worktree) int {
	if w.Main {
		return 0
	}
	return f.Metas[w.Branch()].Ordinal
}

// ----------------------------------------------------------------------------
// Reconnecting

const (
	backoffFirst = 200 * time.Millisecond
	backoffLimit = 5 * time.Second
)

// A Backoff is the wait before each attempt to subscribe again: the first one after an attempt that reached ready, then twice the last, up to a limit.
type Backoff struct {
	last time.Duration
}

// Next returns the wait before the next attempt, the last one having reached ready or not.
func (b *Backoff) Next(reachedReady bool) time.Duration {
	if reachedReady {
		b.last = 0
	}
	b.last = cmp.Or(min(b.last*2, backoffLimit), backoffFirst)
	return b.last
}
