import { create } from 'zustand'
import type { DomainAnswered } from '@/api/types.gen'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

// answers: the follow-up it opens on (a row's `sync <w>`, a callout's fix, a run's next step), none from the header;
// picking: the header's, every stack with the command's own pick; deciding: it opens on the Worktrees step, a choice
// pending (a worktree named dirty, a selection)
export type SyncAsk = { answers: DomainAnswered[]; left: Left[]; picking: boolean; deciding: boolean }

// a store, not the dialog's state: the menus, the selection bar, the header, the callout and a run's follow-ups open
// the one dialog
export const useSyncDialog = create<SyncAsk & { open: boolean }>(() => ({
  open: false,
  answers: [],
  left: [],
  picking: false,
  deciding: false,
}))

export function openSync(repo: string, ask: SyncAsk) {
  dismissSucceeded(repo, 'sync')
  useSyncDialog.setState({ open: true, ...ask })
}
