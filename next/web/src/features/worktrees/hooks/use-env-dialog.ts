import { create } from 'zustand'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

// the worktree on screen, checked beside those the core picks; or the ones a run left to run again, straight to Confirm
export type EnvAsk = { worktree: string; again?: string[] | undefined }

// a store, not the dialog's state: the page header, each key to check and Run again open the one dialog
export const useEnvDialog = create<EnvAsk & { open: boolean }>(() => ({ open: false, worktree: '', again: undefined }))

export function openEnv(repo: string, ask: EnvAsk) {
  dismissSucceeded(repo, 'env')
  useEnvDialog.setState({ open: true, again: undefined, ...ask })
}
