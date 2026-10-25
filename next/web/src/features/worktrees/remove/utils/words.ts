import type { CleanBlocker, CleanRemoval } from '@/api/types.gen'

// The core words its refusals for the command line; the web says each reason its own way, over the blockers the
// schema publishes, so a blocker the core adds fails the build until it has words.
export const blockerWords: Record<CleanBlocker, (removal: CleanRemoval) => string> = {
  locked: () => 'Locked',
  dirty: () => 'Uncommitted changes',
  unpushed: ({ unpushed = 0 }) => `${unpushed} commit${unpushed === 1 ? '' : 's'} no other branch nor remote holds`,
  open_pull_request: ({ pull_request: url = '' }) => `An open pull request: ${url}`,
  jobs_running: ({ jobs = [] }) => `Jobs running: ${jobs.join(', ')}`,
}

// what removing it anyway loses, as one sentence
export const lostWords: Record<CleanBlocker, string> = {
  locked: 'its lock is lifted',
  dirty: 'its uncommitted changes are lost',
  unpushed: 'its commits no other branch holds are lost',
  open_pull_request: 'its pull request stays open on GitHub',
  jobs_running: 'its jobs keep running',
}
