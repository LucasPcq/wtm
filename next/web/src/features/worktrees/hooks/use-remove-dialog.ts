import { create } from 'zustand'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

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
