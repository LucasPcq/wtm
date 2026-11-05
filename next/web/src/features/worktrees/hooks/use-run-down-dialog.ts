import { toast } from 'sonner'
import { create } from 'zustand'
import type { RundownPlan } from '@/api/types.gen'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

export type RunDownStep = 'worktrees' | 'confirm'

// picking: opened on the worktrees, so Confirm has a Back; from a worktree its subject is given
export type RunDownAsk = { worktrees: string[]; picking: boolean; left: Left[] }

// ran is the stop under way, for its screen and its toast to say while no envelope has come
export type RunDownRan = { worktrees: string[]; plan: RundownPlan | undefined }

type Dialog = RunDownAsk & { open: boolean; ran: RunDownRan }

const closed: Dialog = { open: false, worktrees: [], picking: true, left: [], ran: { worktrees: [], plan: undefined } }

export const useRunDownDialogs = create<Record<string, Dialog | undefined>>(() => ({}))

export const useRunDownDialog = (repo: string) => useRunDownDialogs((dialogs) => dialogs[repo] ?? closed)

export const runDownDialogOf = (repo: string) => useRunDownDialogs.getState()[repo] ?? closed

export const setRunDownDialog = (repo: string, change: Partial<Dialog>) =>
  useRunDownDialogs.setState((dialogs) => ({ [repo]: { ...(dialogs[repo] ?? closed), ...change } }))

export const toastIdOf = (repo: string) => `run-down:${repo}`

export function showRunDown(repo: string) {
  toast.dismiss(toastIdOf(repo))
  setRunDownDialog(repo, { open: true })
}

// from the page it asks the worktrees; from a worktree it opens on Confirm
export function openRunDown(repo: string, ask: Partial<RunDownAsk> = {}) {
  dismissSucceeded(repo, 'run-down')
  toast.dismiss(toastIdOf(repo))
  const worktrees = ask.worktrees ?? []
  setRunDownDialog(repo, {
    open: true,
    worktrees,
    left: ask.left ?? [],
    picking: ask.picking ?? worktrees.length === 0,
  })
}
