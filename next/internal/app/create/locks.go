package create

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
)

func (r Request) Locks() []operation.Lock {
	return locksOf(slices.DeleteFunc(slices.Clone(r.Branches), func(name string) bool { return name == "" }))
}

func Locks(f Facts, req Request) []operation.Lock {
	return locksOf(append(slices.Clone(req.Branches), f.fastForwarded(req)...))
}

func locksOf(worktrees []string) []operation.Lock {
	locks := []operation.Lock{{Scope: operation.ScopeRepo, Mode: operation.Shared}}
	slices.Sort(worktrees)
	for _, name := range slices.Compact(worktrees) {
		locks = append(locks, operation.Lock{Scope: operation.ScopeWorktree, Name: name, Mode: operation.Exclusive})
	}
	return locks
}
