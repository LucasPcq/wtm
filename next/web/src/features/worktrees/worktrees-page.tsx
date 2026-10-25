import { useQuery } from '@tanstack/react-query'
import { worktreeCount } from '@/api/queries/commands'
import { reposQuery } from '@/api/queries/repos'
import { PageHeader } from '@/components/page-header'
import { Toolbar } from '@/features/worktrees/components/toolbar'
import { CreateButton } from '@/features/worktrees/create/create-button'
import { useUnderWay } from '@/features/worktrees/create/hooks/use-create-run'
import { DetailPanel } from '@/features/worktrees/detail/detail-panel'
import { useRead } from '@/features/worktrees/hooks/use-read'
import { columnIds, defaultColumns } from '@/features/worktrees/list/components/columns'
import { ColumnsPanel } from '@/features/worktrees/list/components/columns-panel'
import { Notes } from '@/features/worktrees/list/components/notes'
import { ListView } from '@/features/worktrees/list/list-view'
import { rowsOf } from '@/features/worktrees/list/utils/rows'
import { RemoveDialog } from '@/features/worktrees/remove/remove-dialog'
import { useChoice } from '@/features/worktrees/store/ui-store'
import { TreeView } from '@/features/worktrees/tree/tree-view'
import { overlaid } from '@/features/worktrees/utils/condition'
import { countsOf } from '@/features/worktrees/utils/filters'
import { unavailable } from '@/features/worktrees/utils/github'
import type { WorktreesSearch } from '@/features/worktrees/utils/search'
import { repoName } from '@/lib/repo'

export function WorktreesPage({ repo, search }: { repo: string; search: WorktreesSearch }) {
  const tree = search.view === 'tree'
  // only the view on screen is read: two reads of git on every event otherwise
  const list = useRead(repo, 'list', !tree)
  const forest = useRead(repo, 'tree', tree)
  const root = useQuery(reposQuery).data?.find((known) => known.id === repo)?.root
  const columns = useChoice('columns', repo, columnIds, defaultColumns)
  const underWay = useUnderWay(repo)

  const githubOut = unavailable(list.github.data?.warnings)
  const githubAsked = list.github.isSuccess && !githubOut
  const rows = list.live.data
    ? rowsOf({
        live: list.live.data,
        github: githubAsked ? list.github.data : undefined,
        readAt: list.live.dataUpdatedAt,
        underWay,
      })
    : []
  const read = tree ? forest.live.data : list.live.data
  const count = read ? worktreeCount(read) : 0

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
        action={
          <div className="flex items-center gap-2">
            <RemoveDialog repo={repo} />
            <CreateButton repo={repo} />
          </div>
        }
      />
      <Toolbar
        search={search}
        {...(!tree && {
          counts: countsOf(rows, githubAsked),
          trailing: <ColumnsPanel shown={columns.shown} onToggle={columns.toggle} onReset={columns.reset} />,
        })}
      />

      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col" aria-busy={!read}>
          <div className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
            {tree ? (
              <TreeView repo={repo} search={search} underWay={underWay} />
            ) : (
              <ListView live={list.live} rows={rows} shown={columns.shown} search={search} />
            )}
          </div>
          {/* below the rows, not after them: the notes stay put whatever the filters keep */}
          {!tree && list.live.data && (
            <Notes warnings={githubOut ? [...list.live.data.warnings, githubOut] : list.live.data.warnings} />
          )}
        </div>
        {search.worktree && (
          <DetailPanel key={search.worktree} repo={repo} worktree={search.worktree} actions={actions()} />
        )}
      </div>
    </>
  )
}
