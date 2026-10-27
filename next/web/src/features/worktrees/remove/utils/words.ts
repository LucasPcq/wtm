import type { TeardownRemoval, WorktreeBlocker } from '@/api/types.gen'
import type { OutcomeKind } from '@/features/worktrees/utils/outcome'

// The core words its refusals for the command line; the web says each reason its own way, over the blockers the
// schema publishes, so a blocker the core adds fails the build until it has words.
export const blockerWords: Record<WorktreeBlocker, (removal: TeardownRemoval) => string> = {
  locked: () => 'Locked',
  dirty: () => 'Uncommitted changes',
  unpushed: ({ unpushed = 0 }) => `${unpushed} commit${unpushed === 1 ? '' : 's'} on no other branch`,
  open_pull_request: () => 'An open pull request',
  jobs_running: ({ jobs = [] }) => `Jobs running: ${jobs.join(', ')}`,
}

// what removing it anyway loses, said where it is decided: on the row once checked
export const lostWords: Record<WorktreeBlocker, (removal: TeardownRemoval) => string> = {
  locked: () => 'Its lock is lifted.',
  dirty: () => 'Its uncommitted changes are lost.',
  unpushed: ({ unpushed = 0 }) =>
    unpushed === 1
      ? 'Its 1 commit on no other branch is lost.'
      : `Its ${unpushed} commits on no other branch are lost.`,
  open_pull_request: () => 'Its pull request stays open on GitHub.',
  jobs_running: ({ jobs = [] }) => `Its jobs ${jobs.join(', ')} keep running.`,
}

export const outcomeWords: Record<OutcomeKind, string> = {
  failed: "Couldn't remove",
  check: 'To check',
  done: 'Removed',
}

// the plan's warnings the Confirm step says elsewhere: the jobs on the row removed anyway, the data in its Details
export const sharedWarnings = { jobsLeft: 'clean.jobs_left', dataKept: 'clean.data_kept' } as const
