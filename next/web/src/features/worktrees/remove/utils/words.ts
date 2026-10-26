import type { CleanBlocker, CleanRemoval } from '@/api/types.gen'
import type { Choice } from '@/features/worktrees/remove/utils/plan'
import type { OutcomeKind } from '@/features/worktrees/utils/outcome'

// The core words its refusals for the command line; the web says each reason its own way, over the blockers the
// schema publishes, so a blocker the core adds fails the build until it has words.
export const blockerWords: Record<CleanBlocker, (removal: CleanRemoval) => string> = {
  locked: () => 'Locked',
  dirty: () => 'Uncommitted changes',
  unpushed: ({ unpushed = 0 }) => `${unpushed} commit${unpushed === 1 ? '' : 's'} no other branch nor remote holds`,
  open_pull_request: () => 'An open pull request',
  jobs_running: ({ jobs = [] }) => `Jobs running: ${jobs.join(', ')}`,
}

// what removing it anyway loses, said once, on the Confirm step
export const lostWords: Record<CleanBlocker, (removal: CleanRemoval) => string> = {
  locked: () => 'Its lock is lifted.',
  dirty: () => 'Its uncommitted changes are lost.',
  unpushed: ({ unpushed = 0 }) => `Its ${unpushed} commit${unpushed === 1 ? '' : 's'} no other branch holds are lost.`,
  open_pull_request: () => 'Its pull request stays open on GitHub.',
  jobs_running: ({ jobs = [] }) => `Its jobs ${jobs.join(', ')} keep running.`,
}

export const choiceWords: Record<Choice, string> = { keep: 'Keep it', remove: 'Remove anyway' }

export const outcomeWords: Record<OutcomeKind, string> = {
  failed: "Couldn't remove",
  check: 'To check',
  done: 'Removed',
}

// the plan's warnings the Confirm step says elsewhere: the jobs on the row removed anyway, the data in its Details
export const sharedWarnings = { jobsLeft: 'clean.jobs_left', dataKept: 'clean.data_kept' } as const
