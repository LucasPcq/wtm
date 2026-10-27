import type { TeardownRemoval } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export const unsafe = (removal: TeardownRemoval) => (removal.blockers ?? []).length > 0

// what makes a worktree unsafe, as one value: a tick given under another no longer holds
export const riskOf = (removal: TeardownRemoval) => (removal.blockers ?? []).join(',')

// a worktree on no branch, or a branch with no worktree, is no "worktree with its branch"
export const plain = (removals: TeardownRemoval[]) => removals.every((removal) => removal.branch && removal.path)

// a worktree the plan leaves as it is has no phase, and so no unit
export const unitsOf = (plan: { worktrees: TeardownRemoval[] | null }): Unit[] =>
  (plan.worktrees ?? []).flatMap(({ worktree, phases }) =>
    (phases ?? []).length > 0 ? [todo(worktree, phases ?? [])] : [],
  )
