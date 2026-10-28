import { create } from 'zustand'
import type { DomainError } from '@/api/types.gen'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

export type ExecStep = 'worktrees' | 'command' | 'confirm'

// the command is only ever one the person typed, or one a run of theirs gives back to run again: never the address's
export type ExecAsk = {
  worktrees: string[]
  left: Left[]
  command: string
  step: ExecStep
  held?: DomainError | undefined
}

// ran is the command of the run under way and how many run at once, for its screen to say while no envelope has
export type Ran = { worktrees: string[]; command: string; jobs: number }

export const useExecDialog = create<ExecAsk & { open: boolean; ran: Ran }>(() => ({
  open: false,
  worktrees: [],
  left: [],
  command: '',
  step: 'worktrees',
  ran: { worktrees: [], command: '', jobs: 1 },
}))

export function openExec(repo: string, { worktrees, left }: { worktrees: string[]; left: Left[] }) {
  dismissSucceeded(repo, 'exec')
  useExecDialog.setState({ open: true, worktrees, left, command: '', step: 'worktrees', held: undefined })
}

// back on Confirm: the plan shows the command again before it runs, under what refused it last, if anything did
export const confirmExec = (ask: { worktrees: string[]; command: string; held?: DomainError | undefined }) =>
  useExecDialog.setState({ open: true, left: [], step: 'confirm', ...ask, held: ask.held })
