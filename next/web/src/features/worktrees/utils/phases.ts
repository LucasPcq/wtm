import type { DomainPhase } from '@/api/types.gen'

export const phaseLabel: Record<DomainPhase, string> = {
  fast_forward: 'Fast-forward the source branch',
  worktree: 'Add the worktree',
  env: 'Copy its .env files',
  meta: 'Record it',
  ordinal: 'Number it',
  ports: 'Settle its ports',
  on_create: 'Run the on_create hooks',
  move: 'Move the changes',
  on_clean: 'Run the on_clean hooks',
  remove: 'Remove the worktree',
}
