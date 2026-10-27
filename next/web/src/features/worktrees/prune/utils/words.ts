import type { RemovalWords } from '@/features/worktrees/components/removal-progress'

export const pruneWords: RemovalWords = {
  title: 'Prune worktrees',
  past: 'pruned',
  noun: 'prune',
  outcome: { failed: "Couldn't prune", check: 'To check', done: 'Pruned' },
  dataKept: 'prune.data_kept',
}

export const notFinished = 'prune.not_finished'
