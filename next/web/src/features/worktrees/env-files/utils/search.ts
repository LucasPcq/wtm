// the worktree whose files the page shows, and whether the values wtm does not write are shown
export type EnvFilesSearch = { worktree?: string | undefined; values?: 'shown' | undefined }

export function validateSearch(search: Record<string, unknown>): EnvFilesSearch {
  return {
    ...(typeof search.worktree === 'string' && search.worktree !== '' && { worktree: search.worktree }),
    ...(search.values === 'shown' && { values: 'shown' as const }),
  }
}
