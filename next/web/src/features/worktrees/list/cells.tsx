import {
  CircleCheck,
  CircleDashed,
  CircleDot,
  CircleSlash,
  CircleX,
  GitMerge,
  LoaderCircle,
  TriangleAlert,
  type LucideIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import type { ListDetail } from '@/api/types.gen'
import { ago } from '@/lib/time'
import { Glyph, type Tone } from '../shared/glyph'
import type { ColumnId } from './columns'
import type { Row } from './rows'
import { changedLabel, stateOf, type State } from './state'

const muted = (text: string) => <span className="text-muted-foreground">{text}</span>
const none = muted('—')

const looks: Record<State, { icon: LucideIcon; tone: Tone; label: (detail: ListDetail) => string }> = {
  missing: { icon: CircleSlash, tone: 'failed', label: () => 'missing' },
  rebasing: { icon: TriangleAlert, tone: 'attention', label: () => 'rebasing' },
  merged: { icon: GitMerge, tone: 'muted', label: () => 'merged' },
  changed: { icon: CircleDashed, tone: 'attention', label: changedLabel },
  clean: { icon: CircleCheck, tone: 'ok', label: () => 'clean' },
}

function stateCell(row: Row, detail: ListDetail) {
  const look = looks[stateOf(detail, row.merged)]
  const label = look.label(detail)
  return (
    <Glyph icon={look.icon} tone={look.tone}>
      {row.merged && row.pr ? `PR #${row.pr.number} ${label}` : label}
    </Glyph>
  )
}

function runningCell(detail: ListDetail) {
  if (detail.services === null) return muted('unknown')
  if (detail.services.length === 0) return none
  return (
    <Glyph icon={CircleDot} tone="ok">
      {detail.services.length} up · {detail.services.join(', ')}
    </Glyph>
  )
}

function behindCell(detail: ListDetail) {
  if (detail.behind === null) return none
  if (detail.behind === 0) return muted('in step')
  return (
    <Glyph icon={TriangleAlert} tone="attention">
      {detail.behind} behind {detail.parent}
    </Glyph>
  )
}

function originCell(detail: ListDetail) {
  if (!detail.origin) return muted('not on origin')
  const { ahead, behind } = detail.origin
  if (ahead === 0 && behind === 0) return muted('in step')
  return (
    <span className="tabular-nums">{[ahead && `↑${ahead}`, behind && `↓${behind}`].filter(Boolean).join(' ')}</span>
  )
}

function changesCell(detail: ListDetail) {
  if (!detail.changes) return none
  const { changed, untracked } = detail.changes
  if (changed + untracked === 0) return muted('none')
  return (
    <span className="tabular-nums">
      {[changed && `${changed} changed`, untracked && `${untracked} untracked`].filter(Boolean).join(' · ')}
    </span>
  )
}

function prCell(row: Row) {
  if (row.pr === undefined) return none
  if (row.pr === null) return muted('none')
  return (
    <a
      href={row.pr.url}
      target="_blank"
      rel="noreferrer"
      className="underline-offset-4 hover:underline"
      onClick={(e) => e.stopPropagation()}
    >
      #{row.pr.number} {row.pr.draft ? 'draft' : row.pr.state}
    </a>
  )
}

function checksCell(row: Row) {
  const checks = row.pr?.checks
  if (!checks) return none
  if (checks.failed > 0)
    return (
      <Glyph icon={CircleX} tone="failed">
        {checks.failed} failed
      </Glyph>
    )
  if (checks.pending > 0)
    return (
      <Glyph icon={LoaderCircle} tone="attention">
        {checks.pending} pending
      </Glyph>
    )
  return (
    <Glyph icon={CircleCheck} tone="ok">
      {checks.passed} passed
    </Glyph>
  )
}

function addressCell(detail: ListDetail) {
  const [first, ...more] = detail.urls ?? []
  if (!first) return none
  return (
    <span className="flex min-w-0 items-center gap-1">
      <a
        href={first}
        target="_blank"
        rel="noreferrer"
        className="truncate underline underline-offset-4"
        onClick={(e) => e.stopPropagation()}
      >
        {first.replace(/^https?:\/\//, '')}
      </a>
      {more.length > 0 && muted(`+${more.length}`)}
    </span>
  )
}

const when = (at: string | null, now: number) => (at ? muted(ago(at, now)) : none)

// cell is what the column shows of row, which was read.
export function cell(column: ColumnId, row: Row, detail: ListDetail, now: number): ReactNode {
  switch (column) {
    case 'state':
      return stateCell(row, detail)
    case 'behind':
      return behindCell(detail)
    case 'origin':
      return originCell(detail)
    case 'changes':
      return changesCell(detail)
    case 'pr':
      return prCell(row)
    case 'checks':
      return checksCell(row)
    case 'running':
      return runningCell(detail)
    case 'address':
      return addressCell(detail)
    case 'active':
      return when(detail.last_commit_at, now)
    case 'parent':
      return detail.parent ? detail.parent : none
    case 'created':
      return when(detail.created_at, now)
    case 'path':
      return <span className="truncate font-mono text-xs text-muted-foreground">{detail.path}</span>
  }
}
