import { create } from 'zustand'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

// picking: the header opens on the worktrees, prechecked by the command; the others open on Confirm, on those named
export type FastForwardAsk = { worktrees: string[]; left: Left[]; picking: boolean }

// a store, not the dialog's state: the menus, the selection bar, the header and the callout open the one dialog
export const useFastForwardDialog = create<FastForwardAsk & { open: boolean }>(() => ({
  open: false,
  worktrees: [],
  left: [],
  picking: false,
}))

export function openFastForward(repo: string, ask: FastForwardAsk) {
  dismissSucceeded(repo, 'fast-forward')
  useFastForwardDialog.setState({ open: true, ...ask })
}
