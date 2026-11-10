import type { EnvPlan, EnvReconcile } from '@/api/types.gen'
import { settledWords } from '@/features/worktrees/env/utils/words'
import { todo, type Unit } from '@/features/worktrees/utils/progress'

// one change of a worktree's files, as Confirm and Done list it
export type Change = { key: string; name: string; file?: string; does: string }

export const writes = (r: EnvReconcile) => (r.phases ?? []).length > 0

// what the core says the apply does to one worktree, a line each: the isolation recorded, its numbering, each file
// rebuilt, each key settled, each value wtm writes
export function changesOf(r: EnvReconcile): Change[] {
  const changes: Change[] = []
  if (r.records) changes.push({ key: 'records', name: `now ${r.records}`, does: 'its isolation recorded' })
  if (!r.numbered) changes.push({ key: 'numbered', name: 'its ports', does: 'numbered, then settled' })
  for (const file of r.files ?? []) {
    if (file.created)
      changes.push({ key: `created:${file.target}`, name: file.target, does: `rebuilt from ${r.strategy}` })
    for (const key of (file.keys ?? []).filter((one) => one.settled))
      changes.push({
        key: `${file.target}:${key.key}`,
        name: key.key,
        file: file.target,
        does: key.restored ? "back to its source's" : settledWords[key.status],
      })
  }
  for (const entry of r.ports.entries ?? []) {
    if (entry.status !== 'rewrite') continue
    const origin = entry.moves?.[0]?.origin
    const to = entry.new_value ?? origin ?? String(entry.resolved)
    const from = entry.current_value ? `${entry.current_value} → ` : ''
    changes.push({ key: `${entry.file}:${entry.key}`, name: entry.key, file: entry.file, does: `${from}${to}` })
  }
  for (const owned of r.ports.owned ?? []) {
    if (owned.changed)
      changes.push({ key: `${owned.file}:${owned.key}`, name: owned.key, file: owned.file, does: 'set by wtm' })
  }
  return changes
}

// the keys a worktree keeps as they are, with why
export const keptOf = (r: EnvReconcile) =>
  (r.files ?? []).flatMap((file) =>
    (file.keys ?? [])
      .filter((key) => !key.settled)
      .map((key) => ({ file: file.target, key: key.key, status: key.status })),
  )

export const unitsOf = (plan: EnvPlan): Unit[] =>
  (plan.worktrees ?? []).filter(writes).map(({ worktree, phases }) => todo(worktree, phases ?? []))
