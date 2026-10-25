import type { CleanBlocker, DomainFieldError } from '@/api/types.gen'

// The core words its refusals for the command line; the web says each reason its own way, over the blockers the
// schema publishes, so a blocker the core adds fails the build until it has words.
export const blockerWords: Record<CleanBlocker, (params: Record<string, string>) => string> = {
  locked: () => 'Locked',
  dirty: () => 'Uncommitted changes',
  unpushed: ({ count }) => `${count} commit${count === '1' ? '' : 's'} no other branch nor remote holds`,
  open_pull_request: ({ url }) => `An open pull request: ${url}`,
  jobs_running: ({ jobs }) => `Jobs running: ${jobs}`,
}

const blockers = Object.keys(blockerWords) as CleanBlocker[]

export const blockerOf = (code: string): CleanBlocker | undefined =>
  blockers.find((blocker) => code === `clean.${blocker}`)

export const reasonOf = (problem: DomainFieldError) => {
  const blocker = blockerOf(problem.code)
  return blocker ? blockerWords[blocker](problem.params ?? {}) : problem.message
}

export const worktreeWords = {
  label: 'Worktrees',
  help: 'Each goes with its branch and its record; the remote branch stays.',
}
