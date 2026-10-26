import { Link } from '@tanstack/react-router'
import {
  type CellData,
  type ColumnDef,
  columnSizingFeature,
  columnVisibilityFeature,
  type RowData,
  type TableFeatures,
  tableFeatures,
} from '@tanstack/react-table'
import { CircleDot, TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import type { ListDetail } from '@/api/types.gen'
import { Checks } from '@/features/worktrees/components/checks'
import { Glyph } from '@/features/worktrees/components/glyph'
import { type Row, unread } from '@/features/worktrees/list/utils/rows'
import { conditionLook } from '@/features/worktrees/utils/condition'
import { ago } from '@/lib/time'
import { withoutScheme } from '@/lib/url'

export const groups = ['Git', 'Review', 'Run', 'Details'] as const

declare module '@tanstack/react-table' {
  interface ColumnMeta<TFeatures extends TableFeatures, TData extends RowData, TValue extends CellData = CellData> {
    hint: string
    group: (typeof groups)[number]
  }
}

export const features = tableFeatures({ columnVisibilityFeature, columnSizingFeature })

const muted = (text: string) => <span className="text-muted-foreground">{text}</span>
const none = muted('—')
const when = (at: string | null, now: number) => (at ? muted(ago(at, now)) : none)

const read =
  (render: (row: Row, detail: ListDetail) => ReactNode) =>
  ({ row }: { row: { original: Row } }) =>
    row.original.detail && render(row.original, row.original.detail)

export const branchColumn: ColumnDef<typeof features, Row> = {
  id: 'branch',
  header: 'Branch',
  enableHiding: false,
  cell: ({ row: { original }, table }) => {
    // alone, the branch says what the first column would have: what is under way, a worktree not read
    const alone = table.getVisibleLeafColumns().length === 1

    const subline = () => {
      const { detail, error, underWay } = original
      if (alone && underWay) return underWay
      if (!detail && alone) return unread(original)
      if (!detail) return error?.message ?? ''
      if (detail.main) return 'main worktree'
      if (!detail.branch) return 'on no branch'
      const from = detail.parent ? `from ${detail.parent}` : 'a root'
      return detail.needs_sync && detail.behind ? `${from} · ${detail.behind} behind` : from
    }

    return (
      <Link
        from="/r/$repo/worktrees"
        search={(search) => ({ ...search, worktree: original.subject })}
        className="flex min-w-0 flex-col rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring after:absolute after:inset-0"
      >
        <span className="truncate font-medium">{original.subject}</span>
        <span className="truncate text-xs text-muted-foreground">{subline()}</span>
      </Link>
    )
  },
}

export const columnIds = [
  'state',
  'behind',
  'origin',
  'changes',
  'pr',
  'checks',
  'running',
  'address',
  'active',
  'parent',
  'created',
  'path',
] as const
export type ColumnId = (typeof columnIds)[number]

export const defaultColumns: ColumnId[] = ['state', 'running', 'active']

type Column = ColumnDef<typeof features, Row> & { id: ColumnId }

export const columns: Column[] = [
  {
    id: 'state',
    header: 'State',
    size: 176,
    meta: { group: 'Git', hint: 'clean, changed, missing, or what git stopped midway' },
    cell: read(({ condition, pr }, detail) => {
      if (!condition) return none

      const look = conditionLook[condition]
      const changed = (detail.changes?.changed ?? 0) + (detail.changes?.untracked ?? 0)
      const label = condition === 'changed' && changed > 0 ? `${changed} changed` : look.label
      return (
        <Glyph icon={look.icon} tone={look.tone}>
          {condition === 'merged' && pr ? `PR #${pr.number} ${label}` : label}
        </Glyph>
      )
    }),
  },
  {
    id: 'behind',
    header: 'Behind',
    size: 176,
    meta: { group: 'Git', hint: 'commits its parent has that it lacks' },
    cell: read((_, { behind, parent }) => {
      if (behind === null) return none
      if (behind === 0) return muted('in step')
      return (
        <Glyph icon={TriangleAlert} tone="attention">
          {behind} behind {parent}
        </Glyph>
      )
    }),
  },
  {
    id: 'origin',
    header: 'Origin',
    size: 112,
    meta: { group: 'Git', hint: 'ahead and behind its remote branch' },
    cell: read((_, { origin }) => {
      if (!origin) return muted('not on origin')
      if (origin.ahead === 0 && origin.behind === 0) return muted('in step')
      const gap = [origin.ahead && `↑${origin.ahead}`, origin.behind && `↓${origin.behind}`]
      return <span className="tabular-nums">{gap.filter(Boolean).join(' ')}</span>
    }),
  },
  {
    id: 'changes',
    header: 'Changes',
    size: 160,
    meta: { group: 'Git', hint: 'changed and untracked files' },
    cell: read((_, { changes }) => {
      if (!changes) return none
      if (changes.changed + changes.untracked === 0) return muted('none')
      const counts = [
        changes.changed && `${changes.changed} changed`,
        changes.untracked && `${changes.untracked} untracked`,
      ]
      return <span className="tabular-nums">{counts.filter(Boolean).join(' · ')}</span>
    }),
  },
  {
    id: 'pr',
    header: 'Pull request',
    size: 128,
    meta: { group: 'Review', hint: 'number and state, as last read' },
    cell: read(({ pr }) => {
      if (pr === undefined) return none
      if (pr === null) return muted('none')
      return (
        // above the row's link, which covers the row to open its worktree
        <a href={pr.url} target="_blank" rel="noreferrer" className="relative z-10 underline-offset-4 hover:underline">
          #{pr.number} {pr.draft ? 'draft' : pr.state}
        </a>
      )
    }),
  },
  {
    id: 'checks',
    header: 'Checks',
    size: 128,
    meta: { group: 'Review', hint: 'CI of an open pull request' },
    cell: read(({ pr }) => (pr?.checks ? <Checks checks={pr.checks} /> : none)),
  },
  {
    id: 'running',
    header: 'Running',
    size: 192,
    meta: { group: 'Run', hint: 'jobs up in this worktree' },
    cell: read((_, { services }) => {
      if (services === null) return muted('unknown')
      if (services.length === 0) return none
      return (
        <Glyph icon={CircleDot} tone="ok">
          {services.length} up · {services.join(', ')}
        </Glyph>
      )
    }),
  },
  {
    id: 'address',
    header: 'Address',
    size: 224,
    meta: { group: 'Run', hint: 'URL of the jobs up' },
    cell: read((_, { urls }) => {
      const [first, ...more] = urls ?? []
      if (!first) return none
      return (
        <span className="flex min-w-0 items-center gap-1">
          <a
            href={first}
            target="_blank"
            rel="noreferrer"
            className="relative z-10 truncate underline underline-offset-4"
          >
            {withoutScheme(first)}
          </a>
          {more.length > 0 && muted(`+${more.length}`)}
        </span>
      )
    }),
  },
  {
    id: 'active',
    header: 'Active',
    size: 128,
    meta: { group: 'Details', hint: 'when its branch last moved' },
    cell: read(({ readAt }, detail) => when(detail.last_commit_at, readAt)),
  },
  {
    id: 'parent',
    header: 'Parent',
    size: 160,
    meta: { group: 'Details', hint: 'the branch it was created from' },
    cell: read((_, { parent }) => parent || none),
  },
  {
    id: 'created',
    header: 'Created',
    size: 128,
    meta: { group: 'Details', hint: 'when wtm created it' },
    cell: read(({ readAt }, detail) => when(detail.created_at, readAt)),
  },
  {
    id: 'path',
    header: 'Path',
    size: 256,
    meta: { group: 'Details', hint: 'its folder on disk' },
    cell: read((_, { path }) => <span className="truncate font-mono text-xs text-muted-foreground">{path}</span>),
  },
]
