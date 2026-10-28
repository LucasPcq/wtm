import type { TeardownRemoval, WorktreeBlocker } from '@/api/types.gen'

// The core words its refusals for the command line; the web says each reason its own way, over the blockers the
// schema publishes, so a blocker the core adds fails the build until it has words.
export const blockerWords: Record<WorktreeBlocker, (removal: TeardownRemoval) => string> = {
  locked: () => 'Locked',
  dirty: () => 'Uncommitted changes',
  unpushed: ({ unpushed = 0 }) => `${unpushed} commit${unpushed === 1 ? '' : 's'} on no other branch`,
  open_pull_request: () => 'An open pull request',
  jobs_running: ({ jobs = [] }) => `Jobs running: ${jobs.join(', ')}`,
}

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
