import { create } from 'zustand'
import type { RunupPlan } from '@/api/types.gen'
import type { Left } from '@/features/worktrees/components/left-item'
import { dismissSucceeded } from '@/features/worktrees/hooks/use-run'

export type RunUpStep = 'worktrees' | 'profile' | 'confirm'

// an empty profile is the one the core picks for the worktrees: run.toml's default
export type RunUpAsk = { worktrees: string[]; profile: string; step: RunUpStep; left: Left[] }

// ran is the start under way, for its screen and its toast to say while no envelope has come
export type RunUpRan = { worktrees: string[]; profile: string; plan: RunupPlan | undefined }

export const useRunUpDialog = create<RunUpAsk & { open: boolean; ran: RunUpRan }>(() => ({
  open: false,
  worktrees: [],
  profile: '',
  step: 'worktrees',
  left: [],
  ran: { worktrees: [], profile: '', plan: undefined },
}))

// from the page it asks the worktrees; from a worktree, or a profile picked in its menu, it opens on Confirm
export function openRunUp(repo: string, ask: Partial<RunUpAsk> = {}) {
  dismissSucceeded(repo, 'run-up')
  const worktrees = ask.worktrees ?? []
  useRunUpDialog.setState({
    open: true,
    worktrees,
    profile: ask.profile ?? '',
    left: ask.left ?? [],
    step: ask.step ?? (worktrees.length > 0 ? 'confirm' : 'worktrees'),
  })
}
