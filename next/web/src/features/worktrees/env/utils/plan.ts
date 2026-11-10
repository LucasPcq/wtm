import type { DomainRecapValue, EnvPlan, EnvReconcile } from '@/api/types.gen'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

export const writes = (r: EnvReconcile) => (r.phases ?? []).length > 0

// the core's recap by worktree: a line per change its apply makes, then each key it leaves with why (note "left")
export function recapLines(plan: EnvPlan): Map<string, DomainRecapValue[]> {
  const lines = new Map<string, DomainRecapValue[]>()
  let worktree = ''
  for (const row of plan.recap ?? []) {
    if (row.key !== 'worktree') continue
    // a worktree is named on its first line alone
    if (row.label) worktree = row.label
    lines.set(worktree, [...(lines.get(worktree) ?? []), ...(row.values ?? [])])
  }
  return lines
}

// a line of the recap as the core spells it, its file apart: "apps/web/.env  PORT → 3020"
export function lineOf(value: DomainRecapValue) {
  const at = value.text.indexOf('  ')
  return at < 0 ? { file: '', change: value.text } : { file: value.text.slice(0, at), change: value.text.slice(at + 2) }
}

// the worktrees the core's recap says are in step
export const inStepOf = (plan: EnvPlan) =>
  (plan.recap ?? [])
    .filter((row) => row.key === 'in_step')
    .flatMap((row) => (row.values ?? []).map(({ value }) => value))

export const unitsOf = (plan: EnvPlan): Unit[] =>
  (plan.worktrees ?? []).filter(writes).map(({ worktree, phases }) => todo(worktree, phases ?? []))
