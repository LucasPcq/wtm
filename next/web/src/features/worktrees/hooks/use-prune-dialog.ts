import { create } from 'zustand'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

// a store, not the dialog's state: the header and the detail's callout open the one dialog of the page; no worktree
// asked is every one prune finds
export const usePruneDialog = create<{ open: boolean; worktrees: string[] }>(() => ({ open: false, worktrees: [] }))

export function openPrune(repo: string, worktrees: string[]) {
  dismissSucceeded(repo, 'prune')
  usePruneDialog.setState({ open: true, worktrees })
}
