import { toast } from 'sonner'
import { create } from 'zustand'
import type { RunupPlan } from '@/api/types.gen'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

export type RunUpStep = 'worktrees' | 'profile' | 'confirm'

// an empty profile is the one the core picks for the worktrees: run.toml's default
export type RunUpAsk = { worktrees: string[]; profile: string; step: RunUpStep; left: Left[] }

// ran is the start under way, for its screen and its toast to say while no envelope has come
export type RunUpRan = { worktrees: string[]; profile: string; plan: RunupPlan | undefined }

type Dialog = RunUpAsk & { open: boolean; ran: RunUpRan }

const closed: Dialog = {
  open: false,
  worktrees: [],
  profile: '',
  step: 'worktrees',
  left: [],
  ran: { worktrees: [], profile: '', plan: undefined },
}

// one dialog per repository, as one start runs per repository: Show after switching repository opens its own
export const useRunUpDialogs = create<Record<string, Dialog | undefined>>(() => ({}))

export const useRunUpDialog = (repo: string) => useRunUpDialogs((dialogs) => dialogs[repo] ?? closed)

export const runUpDialogOf = (repo: string) => useRunUpDialogs.getState()[repo] ?? closed

export const setRunUpDialog = (repo: string, change: Partial<Dialog>) =>
  useRunUpDialogs.setState((dialogs) => ({ [repo]: { ...(dialogs[repo] ?? closed), ...change } }))

export const toastIdOf = (repo: string) => `run-up:${repo}`

// shown, the dialog says the start under way itself: its toast goes
export function showRunUp(repo: string) {
  toast.dismiss(toastIdOf(repo))
  setRunUpDialog(repo, { open: true })
}

// from the page it asks the worktrees; from a worktree, or a profile picked in its menu, it opens on Confirm
export function openRunUp(repo: string, ask: Partial<RunUpAsk> = {}) {
  dismissSucceeded(repo, 'run-up')
  toast.dismiss(toastIdOf(repo))
  const worktrees = ask.worktrees ?? []
  setRunUpDialog(repo, {
    open: true,
    worktrees,
    profile: ask.profile ?? '',
    left: ask.left ?? [],
    step: ask.step ?? (worktrees.length > 0 ? 'confirm' : 'worktrees'),
  })
}
