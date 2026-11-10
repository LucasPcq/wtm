import type { ProjectIsolation } from '@/api/types.gen'
import type { OutcomeKind } from '@/features/worktrees/utils/outcome'

export const isolationWords: Record<ProjectIsolation, string> = {
  isolated: 'isolated',
  verbatim: 'verbatim: its env is copied as it is',
}

// why the picker offers a worktree it cannot reconcile, by the core's code; another code keeps the core's words
export const problemWords: Record<string, string> = {
  'env.missing': 'its folder is gone',
  'env.unreadable': "its .env files couldn't be read",
}

export const outcomeWords: Record<OutcomeKind, string> = {
  failed: "Couldn't reconcile",
  check: 'To check',
  done: 'Reconciled',
}
