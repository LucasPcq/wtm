import { useQuery } from '@tanstack/react-query'
import { worktreeCount } from '@/api/queries/commands'
import { reposQuery } from '@/api/queries/repos'
import { PageHeader } from '@/components/page-header'
import { HeaderActions } from '@/features/worktrees/components/header-actions'
import { SelectionBar } from '@/features/worktrees/components/selection-bar'
import { Toolbar } from '@/features/worktrees/components/toolbar'
import { CreateButton } from '@/features/worktrees/create/create-button'
import { DetailPanel } from '@/features/worktrees/detail/detail-panel'
import { ExecDialog } from '@/features/worktrees/exec/exec-dialog'
import { FastForwardDialog } from '@/features/worktrees/fast-forward/fast-forward-dialog'
import { useRead } from '@/features/worktrees/hooks/use-read'
import { useUnderWay } from '@/features/worktrees/hooks/use-run'
import { useSelection } from '@/features/worktrees/hooks/use-selection'
import { columnIds, defaultColumns } from '@/features/worktrees/list/components/columns'
import { ColumnsPanel } from '@/features/worktrees/list/components/columns-panel'
import { Notes } from '@/features/worktrees/list/components/notes'
import { ListView } from '@/features/worktrees/list/list-view'
import { rowsOf } from '@/features/worktrees/list/utils/rows'
import { PruneDialog } from '@/features/worktrees/prune/prune-dialog'
import { RemoveDialog } from '@/features/worktrees/remove/remove-dialog'
import { useChoice } from '@/features/worktrees/store/ui-store'
import { TreeView } from '@/features/worktrees/tree/tree-view'
import { type Subject, withPullRequest } from '@/features/worktrees/utils/actions'
import { countsOf, visibleRows } from '@/features/worktrees/utils/filters'
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
  const selection = useSelection(repo)

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
  // the selection is what the rows on screen check: a worktree gone, or filtered out of view, leaves it
  const visible = visibleRows(rows, search)
  const selected = tree ? [] : selection.selected.filter((subject) => visible.some((row) => row.subject === subject))
  const count = read ? worktreeCount(read) : 0

  const subjects = (): Subject[] => {
    if (!tree) return rows.map(({ subject, actions }) => ({ subject, offers: actions }))

    const asked = new Map(forest.github.data?.items.map((item) => [item.subject, item.detail]))
    return (forest.live.data?.items ?? []).flatMap(({ subject, detail }) =>
      detail && !detail.virtual && (detail.actions?.length ?? 0) > 0
        ? [{ subject, offers: withPullRequest(detail.actions ?? [], asked.get(subject)?.actions ?? undefined) }]
        : [],
    )
  }
  const shown = subjects()
  // the selection's actions leave out the rows under way, as their menus do
  const selectedRows = shown.filter(({ subject }) => selected.includes(subject) && underWay[subject] === undefined)

  return (
    <>
      <PageHeader
        title="Worktrees"
        description={read && root && `${count} ${count === 1 ? 'worktree' : 'worktrees'} in ${repoName(root)}`}
        action={
          <div className="flex gap-2">
            <HeaderActions repo={repo} rows={shown.filter(({ subject }) => underWay[subject] === undefined)} />
            <CreateButton repo={repo} />
          </div>
        }
      />
      <RemoveDialog repo={repo} />
      <PruneDialog repo={repo} />
      <FastForwardDialog repo={repo} />
      <ExecDialog repo={repo} />
      {selected.length > 0 ? (
        <SelectionBar repo={repo} count={selected.length} rows={selectedRows} onClear={selection.clear} />
      ) : (
        <Toolbar
          search={search}
          {...(!tree && {
            counts: countsOf(rows, githubAsked),
            trailing: <ColumnsPanel shown={columns.shown} onToggle={columns.toggle} onReset={columns.reset} />,
          })}
        />
      )}

      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col" aria-busy={!read}>
          <div className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
            {tree ? (
              <TreeView repo={repo} search={search} underWay={underWay} subjects={shown} />
            ) : (
              <ListView
                live={list.live}
                rows={rows}
                shown={columns.shown}
                search={search}
                selected={selected}
                onSelect={selection.set}
                repo={repo}
              />
            )}
          </div>
          {/* below the rows, not after them: the notes stay put whatever the filters keep */}
          {!tree && list.live.data && (
            <Notes warnings={githubOut ? [...list.live.data.warnings, githubOut] : list.live.data.warnings} />
          )}
        </div>
        {search.worktree && (
          <DetailPanel
            key={search.worktree}
            repo={repo}
            worktree={search.worktree}
            offers={shown.find(({ subject }) => subject === search.worktree)?.offers ?? []}
            busy={underWay[search.worktree] !== undefined}
          />
        )}
      </div>
    </>
  )
}
