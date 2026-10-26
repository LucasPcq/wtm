import type { CleanPlan, CleanRemoval, DomainAnswered } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export type Choice = 'keep' | 'remove'

export const unsafe = (removal: CleanRemoval) => (removal.blockers ?? []).length > 0

// the request the choices give: the worktrees not kept, forced when an unsafe one is removed anyway
export function requestOf({
  removals,
  choices,
  reparent,
}: {
  removals: CleanRemoval[]
  choices: Record<string, Choice>
  reparent: boolean
}): DomainAnswered[] {
  const going = removals.filter((removal) => choices[removal.worktree] !== 'keep')
  return [
    { path: 'worktree', values: going.map((removal) => removal.worktree) },
    ...(going.some((removal) => unsafe(removal) && choices[removal.worktree] === 'remove')
      ? [{ path: 'force', values: ['true'] }]
      : []),
    ...(reparent ? [{ path: 'reparent_children', values: ['true'] }] : []),
  ]
}

export const unitsOf = (plan: CleanPlan): Unit[] =>
  (plan.worktrees ?? []).map(({ worktree, phases }) => todo(worktree, phases ?? []))
