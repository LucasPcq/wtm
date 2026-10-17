package create

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

func (r Request) Locks() []wtm.Lock {
	return locksOf(slices.DeleteFunc(slices.Clone(r.Branches), func(name string) bool { return name == "" }))
}

func Locks(f Facts, req Request) []wtm.Lock {
	return locksOf(append(slices.Clone(req.Branches), f.fastForwarded(req)...))
}

func locksOf(worktrees []string) []wtm.Lock {
	locks := []wtm.Lock{{Scope: wtm.ScopeRepo, Mode: wtm.LockShared}}
	slices.Sort(worktrees)
	for _, name := range slices.Compact(worktrees) {
		locks = append(locks, wtm.Lock{Scope: wtm.ScopeWorktree, Name: name, Mode: wtm.LockExclusive})
	}
	return locks
}
