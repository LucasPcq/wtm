import type { WorktreeOperation } from '@/api/types.gen'

export const operationWords: Record<WorktreeOperation, string> = {
  rebasing: 'mid-rebase',
  bisecting: 'mid-bisect',
  merging: 'mid-merge',
  cherry_picking: 'mid-cherry-pick',
  reverting: 'mid-revert',
  applying: 'mid-apply',
}
