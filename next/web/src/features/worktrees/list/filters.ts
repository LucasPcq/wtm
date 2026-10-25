import type { Row } from './rows'

export const filters = ['all', 'running', 'needs_sync', 'prunable'] as const
export type Filter = (typeof filters)[number]

export const filterLabel: Record<Filter, string> = {
  all: 'All',
  running: 'Running',
  needs_sync: 'Needs sync',
  prunable: 'Prunable',
}

// matches reports whether row passes filter; prunable is known only once GitHub was asked.
export function matches(row: Row, filter: Filter): boolean {
  switch (filter) {
    case 'all':
      return true
    case 'running':
      return (row.detail?.services?.length ?? 0) > 0
    case 'needs_sync':
      return row.detail?.needs_sync ?? false
    case 'prunable':
      return row.merged ?? false
  }
}

// searched reports whether the branch of row, or its path for one on no branch, holds query, whatever its case.
export function searched(row: Row, query: string): boolean {
  return row.subject.toLowerCase().includes(query.trim().toLowerCase())
}

// countsOf counts the rows each filter keeps; prunable is undefined while GitHub has not been asked.
export function countsOf(rows: Row[], githubAsked: boolean): Record<Filter, number | undefined> {
  const count = (filter: Filter) => rows.filter((row) => matches(row, filter)).length
  return {
    all: rows.length,
    running: count('running'),
    needs_sync: count('needs_sync'),
    prunable: githubAsked ? count('prunable') : undefined,
  }
}
