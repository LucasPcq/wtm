import type { DomainForm, WorktreeCondition } from '@/api/types.gen'
import { refusedWhy } from '@/features/worktrees/utils/reparent'

export type Target = { kind: 'valid' } | { kind: 'parent' } | { kind: 'pending' } | { kind: 'refused'; why: string }

// where a dragged worktree may land is reparent's question of its new parent: a branch it offers takes it, one it
// refuses says why, and a branch it does not know is no branch any more
export function targetOf({
  branch,
  form,
  parent,
  condition,
}: {
  branch: string
  form: DomainForm | undefined
  parent: string | undefined
  condition: WorktreeCondition
}): Target {
  const to = form?.fields.find((field) => field.path === 'to')
  if (to?.kind !== 'select') return { kind: 'pending' }
  if (branch === parent) return { kind: 'parent' }
  if (to.options.some((option) => option.value === branch)) return { kind: 'valid' }

  const refusal = (to.refused ?? []).find((error) => error.params?.value === branch)
  if (refusal) return { kind: 'refused', why: `${branch} ${refusedWhy(refusal)}` }
  return { kind: 'refused', why: condition === 'deleted' ? `${branch} is deleted` : `${branch} is no branch` }
}
