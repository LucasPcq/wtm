import { Link } from '@tanstack/react-router'
import { type ColumnDef, useTable } from '@tanstack/react-table'
import { CircleX, LoaderCircle } from 'lucide-react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { type ColumnId, columnIds, columns, features } from '@/features/worktrees/list/columns'
import type { Row } from '@/features/worktrees/list/rows'
import { Glyph } from '@/features/worktrees/shared/glyph'
import { cn } from '@/lib/utils'

const branch: ColumnDef<typeof features, Row> = {
  id: 'branch',
  header: 'Branch',
  enableHiding: false,
  cell: ({ row: { original } }) => {
    const subline = () => {
      const { detail, error } = original
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

const all = [branch, ...columns]

export function WorktreeTable({ rows, shown, open }: { rows: Row[]; shown: ColumnId[]; open: string | undefined }) {
  const table = useTable({
    features,
    columns: all,
    data: rows,
    getRowId: (row) => row.subject,
    state: { columnVisibility: Object.fromEntries(columnIds.map((id) => [id, shown.includes(id)])) },
  })

  return (
    <Table className="table-fixed">
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id} className="hover:bg-transparent">
            {group.headers.map((header) => (
              <TableHead
                key={header.id}
                className={cn(header.column.id === 'branch' && 'pl-5')}
                style={header.column.id === 'branch' ? undefined : { width: header.getSize() }}
              >
                <table.FlexRender header={header} />
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>

      <TableBody>
        {table.getRowModel().rows.map((row) => {
          const { underWay, detail, status } = row.original
          const [first, ...rest] = row.getVisibleCells()
          const [lead, ...others] = rest

          const leading = () => {
            if (underWay)
              return (
                <Glyph icon={LoaderCircle} tone="busy">
                  {underWay}
                </Glyph>
              )
            if (!detail)
              return (
                <Glyph icon={CircleX} tone={status === 'cancelled' ? 'muted' : 'failed'}>
                  {status === 'cancelled' ? 'not read' : 'unreadable'}
                </Glyph>
              )
            return lead && <table.FlexRender cell={lead} />
          }

          return (
            <TableRow
              key={row.id}
              data-state={row.id === open ? 'selected' : undefined}
              className={cn('relative h-14', underWay && 'opacity-70')}
            >
              {first && (
                <TableCell className="pl-5">
                  <table.FlexRender cell={first} />
                </TableCell>
              )}
              {lead && <TableCell className="truncate">{leading()}</TableCell>}
              {others.map((cell) => (
                <TableCell key={cell.id} className="truncate">
                  <table.FlexRender cell={cell} />
                </TableCell>
              ))}
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
