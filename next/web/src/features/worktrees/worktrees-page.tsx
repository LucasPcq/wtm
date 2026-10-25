import { useQuery } from '@tanstack/react-query'
import { reposQuery } from '@/api/queries/repos'
import { PageHeader } from '@/components/page-header'
import { CreateButton } from '@/features/worktrees/create/create-dialog'
import { DetailPanel } from '@/features/worktrees/detail/detail-panel'
import { useUnderWay } from '@/features/worktrees/hooks/use-create-run'
import { useRead } from '@/features/worktrees/hooks/use-read'
import { columnIds, defaultColumns } from '@/features/worktrees/list/columns'
import { ColumnsPanel } from '@/features/worktrees/list/columns-panel'
import { countsOf } from '@/features/worktrees/list/filters'
import { ListView } from '@/features/worktrees/list/list-view'
import { rowsOf } from '@/features/worktrees/list/rows'
import { Toolbar } from '@/features/worktrees/list/toolbar'
import type { WorktreesSearch } from '@/features/worktrees/search'
import { overlaid } from '@/features/worktrees/shared/condition'
import { unavailable } from '@/features/worktrees/shared/github'
import { TreeView } from '@/features/worktrees/tree/tree-view'
import { useChoice } from '@/features/worktrees/ui-store'
import { repoName } from '@/lib/repo'

export function WorktreesPage({ repo, search }: { repo: string; search: WorktreesSearch }) {
  const tree = search.view === 'tree'
  // only the view on screen is read: two reads of git on every event otherwise
  const list = useRead(repo, 'list', !tree)
  const forest = useRead(repo, 'tree', tree)
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root
  const columns = useChoice('columns', repo, columnIds, defaultColumns)
  const underWay = useUnderWay(repo)

  const rows = list.live.data
    ? rowsOf({ live: list.live.data, github: list.github.data, readAt: list.live.dataUpdatedAt, underWay })
    : []
  const githubAsked = list.github.isSuccess && !unavailable(list.github.data.warnings)
  const worktrees = forest.live.data?.items.filter(({ detail }) => detail && !detail.virtual) ?? []
  const count = tree ? worktrees.length : rows.length
  const read = tree ? forest.live.data : list.live.data

  const actions = () => {
    if (!tree) return rows.find((row) => row.subject === search.worktree)?.actions ?? []

    const live = forest.live.data?.items.find((item) => item.subject === search.worktree)?.detail
    const asked = forest.github.data?.items.find((item) => item.subject === search.worktree)?.detail
    return (live && overlaid(live, asked).actions) ?? []
  }

  return (
    <>
      <PageHeader
        title="Worktrees"
        description={read && root && `${count} ${count === 1 ? 'worktree' : 'worktrees'} in ${repoName(root)}`}
        action={<CreateButton repo={repo} />}
      />
      <Toolbar
        search={search}
        {...(!tree && {
          counts: countsOf(rows, githubAsked),
          trailing: <ColumnsPanel shown={columns.shown} onToggle={columns.toggle} onReset={columns.reset} />,
        })}
      />

      <div className="flex min-h-0 flex-1">
        <div className="min-w-0 flex-1 overflow-y-auto px-3 py-2" aria-busy={!read}>
          {tree ? (
            <TreeView repo={repo} search={search} underWay={underWay} />
          ) : (
            <ListView live={list.live} rows={rows} shown={columns.shown} search={search} />
          )}
        </div>
        {search.worktree && (
          <DetailPanel key={search.worktree} repo={repo} worktree={search.worktree} actions={actions()} />
        )}
      </div>
    </>
  )
}
