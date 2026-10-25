import { useTable } from '@tanstack/react-table'
import { CircleX, LoaderCircle } from 'lucide-react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Glyph } from '@/features/worktrees/components/glyph'
import { branchColumn, type ColumnId, columnIds, columns, features } from '@/features/worktrees/list/components/columns'
import { type Row, unread } from '@/features/worktrees/list/utils/rows'
import { cn } from '@/lib/utils'

const all = [branchColumn, ...columns]

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
                  {unread(row.original)}
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
