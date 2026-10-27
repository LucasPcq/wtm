import type { CleanPlan, DomainAnswered, TeardownRemoval } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export const unsafe = (removal: TeardownRemoval) => (removal.blockers ?? []).length > 0

// what makes a worktree unsafe, as one value: a tick given under another no longer holds
export const riskOf = (removal: TeardownRemoval) => (removal.blockers ?? []).join(',')

// a worktree on no branch, or a branch with no worktree, is no "worktree with its branch"
export const plain = (removals: TeardownRemoval[]) => removals.every((removal) => removal.branch && removal.path)

// the request the checked worktrees give, forced when an unsafe one is among them
export function requestOf({ going, reparent }: { going: TeardownRemoval[]; reparent: boolean }): DomainAnswered[] {
  return [
    { path: 'worktree', values: going.map((removal) => removal.worktree) },
    ...(going.some(unsafe) ? [{ path: 'force', values: ['true'] }] : []),
    ...(reparent ? [{ path: 'reparent_children', values: ['true'] }] : []),
  ]
}

export const unitsOf = (plan: CleanPlan): Unit[] =>
  (plan.worktrees ?? []).map(({ worktree, phases }) => todo(worktree, phases ?? []))
