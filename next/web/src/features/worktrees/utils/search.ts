import { type Filter, filters } from '@/features/worktrees/utils/filters'

export type WorktreesSearch = {
  view?: 'tree' | undefined
  filter?: Filter | undefined
  q?: string | undefined
  worktree?: string | undefined
}

const text = (value: unknown) => (typeof value === 'string' && value !== '' ? value : undefined)

export function validateSearch(search: Record<string, unknown>): WorktreesSearch {
  const filter = filters.find((known) => known !== 'all' && known === search.filter)
  return {
    ...(search.view === 'tree' && { view: 'tree' }),
    ...(filter && { filter }),
    ...(text(search.q) && { q: text(search.q) }),
    ...(text(search.worktree) && { worktree: text(search.worktree) }),
  }
}
