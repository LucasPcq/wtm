// the worktree whose files the page shows
export type EnvFilesSearch = { worktree?: string | undefined }

export function validateSearch(search: Record<string, unknown>): EnvFilesSearch {
  return typeof search.worktree === 'string' && search.worktree !== '' ? { worktree: search.worktree } : {}
}
