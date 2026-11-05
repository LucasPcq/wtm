// the job whose output the page shows: its worktree and its name
export type RunningSearch = { worktree?: string | undefined; job?: string | undefined }

const text = (value: unknown) => (typeof value === 'string' && value !== '' ? value : undefined)

export function validateSearch(search: Record<string, unknown>): RunningSearch {
  return {
    ...(text(search.worktree) && { worktree: text(search.worktree) }),
    ...(text(search.job) && { job: text(search.job) }),
  }
}
