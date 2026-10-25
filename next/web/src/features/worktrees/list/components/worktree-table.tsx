import { useTable } from '@tanstack/react-table'
import { CircleX, LoaderCircle } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Glyph } from '@/features/worktrees/components/glyph'
import { branchColumn, type ColumnId, columnIds, columns, features } from '@/features/worktrees/list/components/columns'
import { type Row, unread } from '@/features/worktrees/list/utils/rows'
import { cn } from '@/lib/utils'

const allColumns = [branchColumn, ...columns]

export function WorktreeTable({
  rows,
  shown,
  open,
  selected,
  onSelect,
}: {
  rows: Row[]
  shown: ColumnId[]
  open: string | undefined
  selected: string[]
  onSelect: (subjects: string[]) => void
}) {
  // the main worktree is never removed, nor checked
  const selectable = rows.filter((row) => !row.detail?.main).map((row) => row.subject)
  const all = selectable.length > 0 && selectable.every((subject) => selected.includes(subject))

  const table = useTable({
    features,
    columns: allColumns,
    data: rows,
    getRowId: (row) => row.subject,
    state: { columnVisibility: Object.fromEntries(columnIds.map((id) => [id, shown.includes(id)])) },
  })

  return (
    <Table className="table-fixed">
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id} className="hover:bg-transparent">
            <TableHead className="w-10 pl-5">
              <Checkbox
                aria-label="Select every worktree"
                checked={all}
                disabled={selectable.length === 0}
                onCheckedChange={(on) => onSelect(on === true ? selectable : [])}
              />
            </TableHead>
            {group.headers.map((header) => (
              <TableHead
                key={header.id}
                className={cn(header.column.id === 'branch' && 'pl-2')}
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
              data-state={row.id === open || selected.includes(row.id) ? 'selected' : undefined}
              className={cn('relative h-14', underWay && 'opacity-70')}
            >
              <TableCell className="pl-5">
                {!detail?.main && (
                  <Checkbox
                    aria-label={`Select ${row.id}`}
                    checked={selected.includes(row.id)}
                    onCheckedChange={(on) =>
                      onSelect(on === true ? [...selected, row.id] : selected.filter((subject) => subject !== row.id))
                    }
                    // the checkbox sits over the row's link: a click checks, it does not open the detail
                    className="relative z-10"
                  />
                )}
              </TableCell>
              {first && (
                <TableCell className="pl-2">
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
