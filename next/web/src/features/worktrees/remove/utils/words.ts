import type { RemovalWords } from '@/features/worktrees/components/removal-progress'

export const removeWords: RemovalWords = {
  title: 'Remove worktrees',
  past: 'removed',
  noun: 'removal',
  outcome: { failed: "Couldn't remove", check: 'To check', done: 'Removed' },
  dataKept: 'clean.data_kept',
}
export const unsafeCode = 'clean.unsafe'
