import { create } from 'zustand'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

// a store, not the dialog's state: the detail's menu and the selection bar open the one dialog of the page
export const useRemoveDialog = create<{ open: boolean; worktrees: string[] }>(() => ({ open: false, worktrees: [] }))

export function openRemove(repo: string, worktrees: string[]) {
  dismissSucceeded(repo, 'clean')
  useRemoveDialog.setState({ open: true, worktrees })
}
