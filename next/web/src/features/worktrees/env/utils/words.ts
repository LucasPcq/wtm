import type { EnvKeyStatus, ProjectIsolation } from '@/api/types.gen'
import type { OutcomeKind } from '@/features/worktrees/utils/outcome'

// a key settled on its source, as Confirm says what it does to it
export const settledWords: Record<EnvKeyStatus, string> = {
  in_step: 'in step',
  add: 'added from its source',
  missing: 'no source gives it a value',
  conflict: "takes its source's value",
  orphan: 'dropped: in no source',
}

// a key left as it is, as Confirm says why
export const leftWords: Record<EnvKeyStatus, string> = {
  in_step: 'in step',
  add: 'not added',
  missing: 'no source gives it a value: write it by hand',
  conflict: 'differs from its source: kept as it is',
  orphan: 'in no source: kept as it is',
}

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
