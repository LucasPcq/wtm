import type { UseQueryResult } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { GitBranchPlus, SearchX } from 'lucide-react'
import type { Read } from '@/api/queries/commands'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { Loading } from '@/features/worktrees/components/loading'
import { ReadFailed } from '@/features/worktrees/components/read-failed'
import type { ColumnId } from '@/features/worktrees/list/components/columns'
import { WorktreeTable } from '@/features/worktrees/list/components/worktree-table'
import type { Row } from '@/features/worktrees/list/utils/rows'
import { visibleRows } from '@/features/worktrees/utils/filters'
import type { WorktreesSearch } from '@/features/worktrees/utils/search'

export function ListView({
  live,
  rows,
  shown,
  search,
  selected,
  onSelect,
}: {
  live: UseQueryResult<Read<'list'>>
  rows: Row[]
  shown: ColumnId[]
  search: WorktreesSearch
  selected: string[]
  onSelect: (subjects: string[]) => void
}) {
  if (live.isPending) return <Loading />
  if (live.isError) return <ReadFailed message={live.error.message} />

  const kept = visibleRows(rows, search)
  const narrowed = (search.filter ?? 'all') !== 'all' || (search.q ?? '') !== ''

  return (
    <>
      {kept.length > 0 && (
        <WorktreeTable rows={kept} shown={shown} open={search.worktree} selected={selected} onSelect={onSelect} />
      )}
      {rows.length === 1 && !narrowed && <OnlyMain />}
      {kept.length === 0 && (rows.length > 1 || narrowed) && <NoMatch />}
    </>
  )
}

function OnlyMain() {
  return (
    <div className="py-16">
      <StatePage icon={<GitBranchPlus />} title="No other worktree yet" command="wtm create feat/my-branch">
        Create one per branch you work on: each gets its own folder, .env and ports, so they run side by side.
      </StatePage>
    </div>
  )
}

function NoMatch() {
  return (
    <div className="py-16">
      <StatePage icon={<SearchX />} title="No worktree matches">
        Nothing here matches the search and the filter.
      </StatePage>
      <div className="flex justify-center">
        <Button variant="outline" asChild>
          <Link from="/r/$repo/worktrees" search={(search) => ({ ...search, q: undefined, filter: undefined })}>
            Clear the search and filters
          </Link>
        </Button>
      </div>
    </div>
  )
}
