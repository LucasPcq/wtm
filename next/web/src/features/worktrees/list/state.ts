import type { ListDetail } from '@/api/types.gen'

// changedLabel says how many files a worktree has changed, its untracked ones included.
export function changedLabel(detail: ListDetail): string {
  const count = (detail.changes?.changed ?? 0) + (detail.changes?.untracked ?? 0)
  return count === 0 ? 'changed' : `${count} changed`
}
