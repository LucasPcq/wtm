import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { GitBranchPlus, Info, SearchX } from 'lucide-react'
import { githubQuery, readQuery } from '@/api/queries/commands'
import type { DomainWarning } from '@/api/types.gen'
import { reposQuery } from '@/api/queries/repos'
import { PageHeader } from '@/components/page-header'
import { StatePage } from '@/components/state-page'
import { Button } from '@/components/ui/button'
import { repoName } from '@/lib/repo'
import { CreateButton } from './create/create-dialog'
import { DetailPanel } from './detail/detail-panel'
import { useStored } from './hooks/use-stored'
import { shownColumns } from './list/columns'
import { ColumnsPanel } from './list/columns-panel'
import { countsOf, matches, searched } from './list/filters'
import { rowsOf } from './list/rows'
import { Toolbar } from './list/toolbar'
import { WorktreeTable } from './list/worktree-table'
import type { WorktreesSearch } from './search'
import { unavailable } from './shared/github'
import { sentence } from './shared/sentence'
import { Loading, ReadFailed } from './shared/loading'
import { useUnderWay } from './hooks/use-create-run'
import { TreeView } from './tree/tree-view'

// Worktrees is the screen of a repository's worktrees: the list or the tree, filtered and searched, and the detail
// of the one open beside it.
export function WorktreesPage({ repo, search }: { repo: string; search: WorktreesSearch }) {
  const live = useQuery(readQuery(repo, 'list'))
  // GitHub is asked once for this screen; Fetch origin asks it again
  const github = useQuery(githubQuery(repo, 'list'))
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root
  const [stored, store] = useStored(`wtm.columns.${repo}`)
  const underWay = useUnderWay(repo)

  const rows = live.data ? rowsOf(live.data, github.data) : []
  // Prunable is known only once GitHub answered
  const counts = countsOf(rows, github.isSuccess && !unavailable(github.data.warnings))
  const filter = search.filter ?? 'all'
  const shown = rows.filter((row) => matches(row, filter) && searched(row, search.q ?? ''))
  const open = rows.find((row) => row.subject === search.worktree)
  const tree = search.view === 'tree'

  return (
    <>
      <PageHeader
        title="Worktrees"
        description={
          live.data && root && `${rows.length} ${rows.length === 1 ? 'worktree' : 'worktrees'} in ${repoName(root)}`
        }
        action={<CreateButton repo={repo} />}
      />
      <Toolbar
        search={search}
        {...(!tree && { counts, trailing: <ColumnsPanel shown={shownColumns(stored)} onChange={store} /> })}
      />
      <div className="flex min-h-0 flex-1">
        <div className="min-w-0 flex-1 overflow-y-auto px-3 py-2" aria-busy={live.isPending}>
          {live.isPending && <Loading />}
          {live.isError && <ReadFailed message={live.error.message} />}
          {live.data && tree && <TreeView repo={repo} search={search} underWay={underWay} />}
          {live.data && !tree && (
            <>
              {shown.length > 0 && (
                <WorktreeTable
                  rows={shown}
                  columns={shownColumns(stored)}
                  open={search.worktree}
                  underWay={underWay}
                  now={live.dataUpdatedAt}
                />
              )}
              <Notes warnings={live.data.warnings} />
              {rows.length === 1 && filter === 'all' && !search.q && <OnlyMain />}
              {shown.length === 0 && (rows.length > 1 || filter !== 'all' || search.q) && <NoMatch />}
            </>
          )}
        </div>
        {search.worktree && (
          <DetailPanel key={search.worktree} repo={repo} worktree={search.worktree} row={open?.detail} />
        )}
      </div>
    </>
  )
}

// Notes are what the read could not tell, said under the list as facts of the moment, not as a failure of it: the
// run daemon that cannot say which jobs are up, a base branch missing, origin out of reach.
function Notes({ warnings }: { warnings: DomainWarning[] }) {
  if (warnings.length === 0) return null
  return (
    <ul className="flex flex-col gap-1 px-5 py-3 text-xs text-muted-foreground">
      {warnings.map((warning) => (
        <li key={warning.code} className="flex items-center gap-1.5">
          <Info className="size-3.5 shrink-0" aria-hidden="true" />
          {sentence(warning.message)}
        </li>
      ))}
    </ul>
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
