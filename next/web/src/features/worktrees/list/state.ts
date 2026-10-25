import type { ListDetail } from '@/api/types.gen'

export type State = 'missing' | 'rebasing' | 'merged' | 'changed' | 'clean'

// stateOf is the one state the State column shows of a worktree, the one that most changes what to do with it
// first: its directory gone, a rebase to finish, its pull request merged, uncommitted changes, else clean.
export function stateOf(detail: ListDetail, merged: boolean | undefined): State {
  if (detail.missing) return 'missing'
  if (detail.rebasing) return 'rebasing'
  if (merged) return 'merged'
  if (detail.dirty) return 'changed'
  return 'clean'
}

// changedLabel says how many files a worktree has changed, its untracked ones included.
export function changedLabel(detail: ListDetail): string {
  const count = (detail.changes?.changed ?? 0) + (detail.changes?.untracked ?? 0)
  return count === 0 ? 'changed' : `${count} changed`
}
