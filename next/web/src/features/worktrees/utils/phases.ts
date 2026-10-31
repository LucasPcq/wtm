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
  exec: 'Run the command',
  rebase: 'Rebase onto its parent',
  push: 'Push to origin',
  job: 'Start the job',
}

export const phaseDuring: Record<DomainPhase, string> = {
  fast_forward: 'the fast-forward of the source branch',
  worktree: 'the add of the worktree',
  env: 'the copy of its .env files',
  meta: 'its record',
  ordinal: 'its numbering',
  ports: 'the settling of its ports',
  on_create: 'the on_create hooks',
  move: 'the move of the changes',
  on_clean: 'the on_clean hooks',
  remove: 'the removal of the worktree',
  exec: 'the command',
  rebase: 'the rebase onto its parent',
  push: 'the push to origin',
  job: 'the start of the job',
}
