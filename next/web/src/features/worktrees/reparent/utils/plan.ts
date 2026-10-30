import type { Applied } from '@/api/queries/commands'
import type { DomainFollowUp, ReparentMove, ReparentPlan } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export const unchanged = (move: Pick<ReparentMove, 'from' | 'to'>) => move.from === move.to

export const unitsOf = (plan: ReparentPlan): Unit[] =>
  (plan.moves ?? []).filter((move) => !unchanged(move)).map((move) => todo(move.branch, []))

const answerOf = (followUp: DomainFollowUp, path: string) =>
  followUp.answers?.find((answer) => answer.path === path)?.values ?? []

// reparent moves its worktrees onto one parent: follow-ups onto several parents are one run per parent, in their order
export function byParent(followUps: DomainFollowUp[]): { to: string; worktrees: string[] }[] {
  const runs = new Map<string, string[]>()
  for (const followUp of followUps) {
    const [to] = answerOf(followUp, 'to')
    if (to === undefined) continue
    runs.set(to, [...(runs.get(to) ?? []), ...answerOf(followUp, 'worktree')])
  }
  return [...runs].map(([to, worktrees]) => ({ to, worktrees }))
}

export const moved = (envelopes: Applied<'reparent'>[]) =>
  envelopes.flatMap((envelope) =>
    envelope.items.filter((item) => item.status === 'done' && item.reason !== 'unchanged'),
  )

export const undosOf = (envelopes: Applied<'reparent'>[]) =>
  envelopes.flatMap((envelope) => envelope.follow_ups.filter((followUp) => followUp.code === 'reparent.undo'))

// runs one after the other each name their own sync: the person rebases them all with one
export function syncOf(envelopes: Applied<'reparent'>[]): DomainFollowUp | undefined {
  const syncs = envelopes.flatMap((envelope) =>
    envelope.follow_ups.filter((followUp) => followUp.code === 'reparent.sync'),
  )
  const [first] = syncs
  if (!first) return undefined
  return { ...first, answers: [{ path: 'worktree', values: syncs.flatMap((sync) => answerOf(sync, 'worktree')) }] }
}

// what a chain that ends at this run has told no one: the runs before it and this one, when no toast said them (the
// dialog open on this run shows only this one); nothing while the chain goes on
export function unreported({
  ran,
  envelope,
  told,
  goesOn,
}: {
  ran: Applied<'reparent'>[]
  envelope: Applied<'reparent'> | undefined
  told: boolean
  goesOn: boolean
}): Applied<'reparent'>[] {
  if (told || goesOn || ran.length === 0) return []
  return envelope ? [...ran, envelope] : ran
}
