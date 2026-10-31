import type { DomainAnswered, SyncPlan, SyncRebase } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export type Switches = { descendants: boolean; push: boolean; keep_conflict: boolean; ff_parents: boolean }

export const unsafe = (rebase: SyncRebase) => (rebase.blockers ?? []).length > 0

// what makes a worktree unsafe, as one value: a tick given under another no longer holds
export const riskOf = (rebase: SyncRebase) => (rebase.blockers ?? []).join(',')

export const moves = (row: { phases: unknown[] | null }) => (row.phases ?? []).length > 0

const rebases = (rebase: SyncRebase) => (rebase.phases ?? []).includes('rebase')

// the count rule: the numbers, "k of n", the primary and "n synced" count rebases, never the fast-forward first
export const rebasesOf = (plan: SyncPlan) => (plan.worktrees ?? []).filter(rebases)

// a plan that only pushes (a run's "push them") says so: it rebases nothing
export const pushOnly = (plan: SyncPlan) => rebasesOf(plan).length === 0 && (plan.worktrees ?? []).some(moves)

export const valuesOf = (answers: DomainAnswered[], path: string) =>
  answers.find((answer) => answer.path === path)?.values

export const switchesOf = (answers: DomainAnswered[]): Switches => ({
  descendants: valuesOf(answers, 'descendants')?.[0] !== 'false',
  push: valuesOf(answers, 'push')?.[0] === 'true',
  keep_conflict: valuesOf(answers, 'keep_conflict')?.[0] === 'true',
  ff_parents: valuesOf(answers, 'ff_parents')?.[0] !== 'false',
})

// chosen, the command picks the worktrees itself and keeps a dirty one; named, those checked go, forced only when the
// person checked a dirty one under that very risk. A switch is sent only off its default
export function requestOf({
  chosen,
  named,
  forced,
  switches,
}: {
  chosen: boolean
  named: string[]
  forced: boolean
  switches: Switches
}): DomainAnswered[] {
  return [
    ...(chosen ? [] : [{ path: 'worktree', values: named }]),
    ...(forced ? [{ path: 'force', values: ['true'] }] : []),
    ...(switches.descendants ? [] : [{ path: 'descendants', values: ['false'] }]),
    ...(switches.ff_parents ? [] : [{ path: 'ff_parents', values: ['false'] }]),
    ...(switches.push ? [{ path: 'push', values: ['true'] }] : []),
    ...(switches.keep_conflict ? [{ path: 'keep_conflict', values: ['true'] }] : []),
  ]
}

// a plan made again on the review's facts reads origin as the review fetched it: one fetch per flow
export const noFetch: DomainAnswered = { path: 'no_fetch', values: ['true'] }

// in run order: the branches fast-forwarded first, then each worktree, parents first
export const unitsOf = (plan: SyncPlan): Unit[] => [
  ...(plan.onto ?? []).filter(moves).map(({ branch, phases }) => todo(branch, phases ?? [])),
  ...(plan.worktrees ?? []).filter(moves).map(({ worktree, phases, parent }) => todo(worktree, phases ?? [], parent)),
]

// the review's rows, each a descendant when the review or the plan says so: the review of every stack (`all`) marks
// none, and a descendant checked anyway is named, which the plan no longer marks
export const markedOf = (plan: SyncPlan | undefined, review: SyncRebase[]): SyncRebase[] =>
  review.map((row) => ({
    ...row,
    descendant: row.descendant || (plan?.worktrees ?? []).some((r) => r.worktree === row.worktree && r.descendant),
  }))

// the worktree a descendant follows: the first of its parents the review does not mark a descendant
export function rootOf(rows: SyncRebase[], worktree: string): string {
  const byName = new Map(rows.map((rebase) => [rebase.worktree, rebase]))
  let at = byName.get(worktree)
  while (at?.descendant) {
    const parent = byName.get(at.parent)
    if (!parent) return at.parent
    at = parent
  }
  return at?.worktree ?? worktree
}
