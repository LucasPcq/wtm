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

// a line of the recap with the file the core names in its params apart, the change as the core spells it after it
export function lineOf(value: DomainRecapValue) {
  const file = value.params?.file ?? ''
  const change = file && value.text.startsWith(file) ? value.text.slice(file.length).trimStart() : value.text
  return { file, change }
}

// the worktrees the core's recap says are in step
export const inStepOf = (plan: EnvPlan) =>
  (plan.recap ?? [])
    .filter((row) => row.key === 'in_step')
    .flatMap((row) => (row.values ?? []).map(({ value }) => value))

export const unitsOf = (plan: EnvPlan): Unit[] =>
  (plan.worktrees ?? []).filter(writes).map(({ worktree, phases }) => todo(worktree, phases ?? []))
