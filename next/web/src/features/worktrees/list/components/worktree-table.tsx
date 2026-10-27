import { useTable } from '@tanstack/react-table'
import { CircleX, LoaderCircle } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Glyph } from '@/features/worktrees/components/glyph'
import { RowMenu } from '@/features/worktrees/components/row-menu'
import { TriCheckbox } from '@/features/worktrees/components/tri-checkbox'
import { branchColumn, type ColumnId, columnIds, columns, features } from '@/features/worktrees/list/components/columns'
import { type Row, unread } from '@/features/worktrees/list/utils/rows'
import type { Subject } from '@/features/worktrees/utils/actions'
import { cn } from '@/lib/utils'

const allColumns = [branchColumn, ...columns]

export function WorktreeTable({
  repo,
  rows,
  shown,
  open,
  selected,
  onSelect,
}: {
  repo: string
  rows: Row[]
  shown: ColumnId[]
  open: string | undefined
  selected: string[]
  onSelect: (subjects: string[]) => void
}) {
  // a worktree a run is working on is not checked: what it would act on is under way
  const checkable = (row: Row) => !row.underWay
  const selectable = rows.filter(checkable).map((row) => row.subject)
  const some = selectable.filter((subject) => selected.includes(subject)).length
  const all = () => {
    if (some === 0) return false
    return some === selectable.length ? true : 'indeterminate'
  }

  const subjectOf = (row: Row): Subject => ({ subject: row.subject, offers: row.actions })
  // a menu on a row of a selection acts on the selection, but for the rows under way
  const menuRows = (row: Row) => {
    const taken = rows.filter((one) => selected.includes(one.subject) && !one.underWay)
    return taken.length > 1 && taken.includes(row) ? taken.map(subjectOf) : [subjectOf(row)]
  }

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
              <TriCheckbox
                label="Select every worktree"
                checked={all()}
                disabled={selectable.length === 0}
                onToggle={() => onSelect(some === selectable.length ? [] : selectable)}
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
            <TableHead className="w-12" />
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
            <RowMenu
              key={row.id}
              repo={repo}
              subject={row.id}
              rows={menuRows(row.original)}
              busy={Boolean(underWay) || !detail}
            >
              {(more) => (
                <TableRow
                  data-state={row.id === open || selected.includes(row.id) ? 'selected' : undefined}
                  className={cn('group/row relative h-14', underWay && 'opacity-70')}
                >
                  <TableCell className="pl-5">
                    {checkable(row.original) && (
                      <Checkbox
                        aria-label={`Select ${row.id}`}
                        checked={selected.includes(row.id)}
                        onCheckedChange={(on) =>
                          onSelect(
                            on === true ? [...selected, row.id] : selected.filter((subject) => subject !== row.id),
                          )
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
                  <TableCell className="pr-3 text-right">{more}</TableCell>
                </TableRow>
              )}
            </RowMenu>
          )
        })}
      </TableBody>
    </Table>
  )
}
