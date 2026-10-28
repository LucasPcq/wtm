import { create } from 'zustand'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

// picking: a selection opens on its worktrees; one worktree opens on its new parent, and with a parent given, on Confirm
export type ReparentAsk = { worktrees: string[]; left: Left[]; picking: boolean; to?: string | undefined }

// a store, not the dialog's state: the menus, the selection bar and the runs at once open the one dialog
export const useReparentDialog = create<ReparentAsk & { open: boolean }>(() => ({
  open: false,
  worktrees: [],
  left: [],
  picking: false,
}))

export function openReparent(repo: string, ask: ReparentAsk) {
  dismissSucceeded(repo, 'reparent')
  useReparentDialog.setState({ open: true, to: undefined, ...ask })
}
