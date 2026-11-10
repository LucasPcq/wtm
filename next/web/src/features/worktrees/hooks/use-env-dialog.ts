import { create } from 'zustand'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

// the worktree on screen, checked beside those the core picks
export type EnvAsk = { worktree: string }

// a store, not the dialog's state: the page header and each key to check open the one dialog
export const useEnvDialog = create<EnvAsk & { open: boolean }>(() => ({ open: false, worktree: '' }))

export function openEnv(repo: string, ask: EnvAsk) {
  dismissSucceeded(repo, 'env')
  useEnvDialog.setState({ open: true, ...ask })
}
