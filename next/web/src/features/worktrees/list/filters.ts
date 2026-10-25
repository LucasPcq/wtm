import type { Row } from '@/features/worktrees/list/rows'

export const filters = ['all', 'running', 'needs_sync', 'prunable'] as const
export type Filter = (typeof filters)[number]

export const filterLabel: Record<Filter, string> = {
  all: 'All',
  running: 'Running',
  needs_sync: 'Needs sync',
  prunable: 'Prunable',
}

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

export const searched = (subject: string, query: string) => subject.toLowerCase().includes(query.trim().toLowerCase())

export function countsOf(rows: Row[], githubAsked: boolean): Record<Filter, number | undefined> {
  const count = (filter: Filter) => rows.filter((row) => matches(row, filter)).length
  return {
    all: rows.length,
    running: count('running'),
    needs_sync: count('needs_sync'),
    prunable: githubAsked ? count('prunable') : undefined,
  }
}
