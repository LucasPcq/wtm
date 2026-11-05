import { create } from 'zustand'
import type { JobAt } from '@/features/worktrees/utils/job-told'

// the outcome of a job's direct stop, opened from its toast's Details: ran is the stop it shows
type Dialog = { open: boolean; ran: JobAt }

const closed: Dialog = { open: false, ran: { worktree: '', job: '' } }

export const useRunStopDialogs = create<Record<string, Dialog | undefined>>(() => ({}))

export const useRunStopDialog = (repo: string) => useRunStopDialogs((dialogs) => dialogs[repo] ?? closed)

export const setRunStopDialog = (repo: string, change: Partial<Dialog>) =>
  useRunStopDialogs.setState((dialogs) => ({ [repo]: { ...(dialogs[repo] ?? closed), ...change } }))

export const showRunStop = (repo: string) => setRunStopDialog(repo, { open: true })
