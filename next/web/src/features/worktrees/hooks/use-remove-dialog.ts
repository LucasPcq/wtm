import { create } from 'zustand'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

export type Left = { worktree: string; why: string }

// what Remove opens on: its worktrees, the safe ones checked when prechecked, and those asked that it leaves as they are
export type RemoveAsk = { worktrees: string[]; left: Left[]; prechecked: boolean }

// a store, not the dialog's state: the menus, the selection bar and the header open the one dialog of the page
export const useRemoveDialog = create<RemoveAsk & { open: boolean }>(() => ({
  open: false,
  worktrees: [],
  left: [],
  prechecked: true,
}))

export function openRemove(repo: string, ask: RemoveAsk) {
  dismissSucceeded(repo, 'clean')
  useRemoveDialog.setState({ open: true, ...ask })
}
