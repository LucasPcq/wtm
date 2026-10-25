import type { DomainPhase } from '@/api/types.gen'

export type Phase = DomainPhase

// How each phase of an operation is named on screen; a phase the core adds fails the build until it is named here.
export const phaseLabel: Record<Phase, string> = {
  fast_forward: 'Fast-forward the source branch',
  worktree: 'Add the worktree',
  env: 'Copy its .env files',
  meta: 'Record it',
  ordinal: 'Number it',
  ports: 'Settle its ports',
  on_create: 'Run the on_create hooks',
  move: 'Move the changes',
}
