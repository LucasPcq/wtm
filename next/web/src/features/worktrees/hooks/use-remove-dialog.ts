import { create } from 'zustand'

// a store, not the dialog's state: the detail's menu and the selection bar open the one dialog of the page
export const useRemoveDialog = create<{ open: boolean; worktrees: string[] }>(() => ({ open: false, worktrees: [] }))

export const openRemove = (worktrees: string[]) => useRemoveDialog.setState({ open: true, worktrees })
