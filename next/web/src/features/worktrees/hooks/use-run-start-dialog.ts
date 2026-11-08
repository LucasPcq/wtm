import { toast } from 'sonner'
import { create } from 'zustand'
import type { RunstartPlan } from '@/api/types.gen'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

export type RunStartStep = 'job' | 'confirm'

// a worktree's job to start, or to start again (restart); from a worktree's menu it asks the job, from a job its Confirm
export type RunStartAsk = { worktree: string; job: string; restart: boolean; step: RunStartStep }

// ran is the start under way, the dialog's or a job's direct one, for its screen to say while no envelope has come
export type RunStartRan = { worktree: string; job: string; restart: boolean; plan: RunstartPlan | undefined }

type Dialog = RunStartAsk & { open: boolean; ran: RunStartRan }

const closed: Dialog = {
  open: false,
  worktree: '',
  job: '',
  restart: false,
  step: 'job',
  ran: { worktree: '', job: '', restart: false, plan: undefined },
}

export const useRunStartDialogs = create<Record<string, Dialog | undefined>>(() => ({}))

export const useRunStartDialog = (repo: string) => useRunStartDialogs((dialogs) => dialogs[repo] ?? closed)

export const runStartDialogOf = (repo: string) => useRunStartDialogs.getState()[repo] ?? closed

export const setRunStartDialog = (repo: string, change: Partial<Dialog>) =>
  useRunStartDialogs.setState((dialogs) => ({ [repo]: { ...(dialogs[repo] ?? closed), ...change } }))

export const toastIdOf = (repo: string) => `run-start:${repo}`

export function showRunStart(repo: string) {
  toast.dismiss(toastIdOf(repo))
  setRunStartDialog(repo, { open: true })
}

export function openRunStart(repo: string, ask: Pick<RunStartAsk, 'worktree'> & Partial<RunStartAsk>) {
  dismissSucceeded(repo, 'run-start')
  toast.dismiss(toastIdOf(repo))
  const job = ask.job ?? ''
  setRunStartDialog(repo, {
    open: true,
    worktree: ask.worktree,
    job,
    restart: ask.restart ?? false,
    step: ask.step ?? (job === '' ? 'job' : 'confirm'),
  })
}
