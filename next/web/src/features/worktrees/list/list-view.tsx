import type { UseQueryResult } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { GitBranchPlus, SearchX } from 'lucide-react'
import type { Read } from '@/api/queries/commands'
import type { DomainWarning } from '@/api/types.gen'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { Loading } from '@/features/worktrees/components/loading'
import { ReadFailed } from '@/features/worktrees/components/read-failed'
import type { ColumnId } from '@/features/worktrees/list/components/columns'
import { Notes } from '@/features/worktrees/list/components/notes'
import { WorktreeTable } from '@/features/worktrees/list/components/worktree-table'
import type { Row } from '@/features/worktrees/list/utils/rows'
import { matches, searched } from '@/features/worktrees/utils/filters'
import type { WorktreesSearch } from '@/features/worktrees/utils/search'

export function ListView({
  live,
  githubOut,
  rows,
  shown,
  search,
}: {
  live: UseQueryResult<Read<'list'>>
  githubOut: DomainWarning | undefined
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
      <Notes warnings={githubOut ? [...live.data.warnings, githubOut] : live.data.warnings} />
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
