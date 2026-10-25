import { Link } from '@tanstack/react-router'
import { CircleX } from 'lucide-react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { cn } from '@/lib/utils'
import { Glyph } from '../shared/glyph'
import { UnderWay } from '../shared/under-way'
import { cell } from './cells'
import { columnText, type ColumnId } from './columns'
import type { Row } from './rows'

const width: Record<ColumnId, string> = {
  state: 'w-44',
  behind: 'w-44',
  origin: 'w-28',
  changes: 'w-40',
  pr: 'w-32',
  checks: 'w-32',
  running: 'w-48',
  address: 'w-56',
  active: 'w-32',
  parent: 'w-40',
  created: 'w-32',
  path: 'w-64',
}

// subline is the line under a branch: where it comes from, and how far behind it is when a sync is due.
function subline(row: Row): string {
  const detail = row.detail
  if (!detail) return row.error?.message ?? ''
  if (detail.main) return 'main worktree'
  if (!detail.branch) return 'on no branch'
  const from = detail.parent ? `from ${detail.parent}` : 'a root'
  return detail.needs_sync && detail.behind ? `${from} · ${detail.behind} behind` : from
}

// WorktreeTable is the list of worktrees with the columns chosen, its times said as of now (when it was read), the one open in the detail marked; a worktree an
// operation of this page works on is dimmed and says what is under way.
export function WorktreeTable(props: {
  rows: Row[]
  columns: ColumnId[]
  open: string | undefined
  underWay: Record<string, string>
  now: number
}) {
  const now = props.now
  return (
    <Table className="table-fixed">
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="pl-5">Branch</TableHead>
          {props.columns.map((column) => (
            <TableHead key={column} className={width[column]}>
              {columnText[column].label}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.rows.map((row) => {
          const underWay = props.underWay[row.subject]
          return (
            <TableRow
              key={row.subject}
              data-state={row.subject === props.open ? 'selected' : undefined}
              className={cn('relative h-14', underWay && 'opacity-70')}
            >
              <TableCell className="pl-5">
                <Link
                  from="/r/$repo/worktrees"
                  search={(search) => ({ ...search, worktree: row.subject })}
                  className="flex min-w-0 flex-col rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring after:absolute after:inset-0"
                >
                  <span className="truncate font-medium">{row.subject}</span>
                  <span className="truncate text-xs text-muted-foreground">{subline(row)}</span>
                </Link>
              </TableCell>
              {props.columns.map((column, i) => (
                <TableCell key={column} className="truncate">
                  {i === 0 && underWay ? (
                    <UnderWay what={underWay} />
                  ) : row.detail ? (
                    cell(column, row, row.detail, now)
                  ) : (
                    i === 0 && (
                      <Glyph icon={CircleX} tone="failed">
                        unreadable
                      </Glyph>
                    )
                  )}
                </TableCell>
              ))}
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
