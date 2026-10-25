import { filters, type Filter } from './list/filters'

// What the address of the Worktrees screen holds, so that a view, a filter or a worktree opened is a link: the tree
// rather than the list, the filter and the search, the worktree whose detail is open.
export type WorktreesSearch = {
  view?: 'tree' | undefined
  filter?: Filter | undefined
  q?: string | undefined
  worktree?: string | undefined
}

const text = (value: unknown) => (typeof value === 'string' && value !== '' ? value : undefined)

// validateSearch keeps of an address what it means, and drops the rest.
export function validateSearch(search: Record<string, unknown>): WorktreesSearch {
  const filter = filters.find((known) => known !== 'all' && known === search.filter)
  return {
    ...(search.view === 'tree' && { view: 'tree' }),
    ...(filter && { filter }),
    ...(text(search.q) && { q: text(search.q) }),
    ...(text(search.worktree) && { worktree: text(search.worktree) }),
  }
}
