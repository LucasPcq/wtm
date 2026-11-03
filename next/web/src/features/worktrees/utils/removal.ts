import type { TeardownRemoval } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export const unsafe = (removal: TeardownRemoval) => (removal.blockers ?? []).length > 0

// what makes a worktree unsafe, as one value: a tick given under another no longer holds
export const riskOf = (removal: TeardownRemoval) => (removal.blockers ?? []).join(',')

// a worktree on no branch, or a branch with no worktree, is no "worktree with its branch"
export const plain = (removals: TeardownRemoval[]) => removals.every((removal) => removal.branch && removal.path)

// the shared services down that the data of removals waits on, once each: those a run may start to drop it now
export const downOf = (removals: TeardownRemoval[]) => [
  ...new Set(
    removals.flatMap((removal) =>
      (removal.data ?? []).filter((n) => n.status === 'defer' || n.status === 'start').map((n) => n.job),
    ),
  ),
]

export const kept = (removal: TeardownRemoval) => (removal.phases ?? []).length === 0

export const unitsOf = (plan: { worktrees: TeardownRemoval[] | null }): Unit[] =>
  (plan.worktrees ?? []).flatMap((removal) => (kept(removal) ? [] : [todo(removal.worktree, removal.phases ?? [])]))
