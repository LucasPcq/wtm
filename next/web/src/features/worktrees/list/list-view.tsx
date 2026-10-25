import type { UseQueryResult } from '@tanstack/react-query'
import type { Read } from '@/api/queries/commands'
import type { ColumnId } from '@/features/worktrees/list/columns'
import { NoMatch, OnlyMain } from '@/features/worktrees/list/empty-states'
import { matches, searched } from '@/features/worktrees/list/filters'
import { Notes } from '@/features/worktrees/list/notes'
import type { Row } from '@/features/worktrees/list/rows'
import { WorktreeTable } from '@/features/worktrees/list/worktree-table'
import type { WorktreesSearch } from '@/features/worktrees/search'
import { Loading, ReadFailed } from '@/features/worktrees/shared/loading'

export function ListView({
  live,
  rows,
  shown,
  search,
}: {
  live: UseQueryResult<Read<'list'>>
  rows: Row[]
  shown: ColumnId[]
  search: WorktreesSearch
}) {
  if (live.isPending) return <Loading />
  if (live.isError) return <ReadFailed message={live.error.message} />

  const filter = search.filter ?? 'all'
  const query = search.q ?? ''
  const kept = rows.filter((row) => matches(row, filter) && searched(row.subject, query))
  const narrowed = filter !== 'all' || query !== ''

  return (
    <>
      {kept.length > 0 && <WorktreeTable rows={kept} shown={shown} open={search.worktree} />}
      <Notes warnings={live.data.warnings} />
      {rows.length === 1 && !narrowed && <OnlyMain />}
      {kept.length === 0 && (rows.length > 1 || narrowed) && <NoMatch />}
    </>
  )
}
