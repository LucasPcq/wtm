import type { DomainAnswered, FastforwardAdvance, FastforwardPlan } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export const unsafe = (advance: FastforwardAdvance) => (advance.blockers ?? []).length > 0

// what makes a worktree unsafe, as one value: a tick given under another no longer holds
export const riskOf = (advance: FastforwardAdvance) => (advance.blockers ?? []).join(',')

export const moves = (advance: FastforwardAdvance) => (advance.phases ?? []).length > 0

// only a worktree behind origin has anything to take: the person decides it, the others are left as they are
export const behind = (advance: FastforwardAdvance) => advance.standing === 'behind'

// chosen, the command picks the worktrees itself and keeps an unsafe one; named, the worktrees left as they are go
// with those checked, so the outcome says why each stayed, and force only when the person checked an unsafe one. Only
// a worktree behind origin can be unsafe, so none of those goes unless checked: forced, it would move unasked
export function requestOf({
  chosen,
  going,
  left,
}: {
  chosen: boolean
  going: FastforwardAdvance[]
  left: FastforwardAdvance[]
}): DomainAnswered[] {
  if (chosen) return []
  return [
    {
      path: 'worktree',
      values: [...going, ...left.filter((advance) => !behind(advance))].map((advance) => advance.worktree),
    },
    ...(going.some(unsafe) ? [{ path: 'force', values: ['true'] }] : []),
  ]
}

export const unitsOf = (plan: FastforwardPlan): Unit[] =>
  (plan.worktrees ?? []).filter(moves).map(({ worktree, phases }) => todo(worktree, phases ?? []))

// a plan made again on the same facts reads origin as the review fetched it: one fetch per flow
export const noFetch: DomainAnswered = { path: 'no_fetch', values: ['true'] }
