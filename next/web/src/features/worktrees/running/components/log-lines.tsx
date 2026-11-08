import { memo, useCallback } from 'react'
import { Alert } from '@/features/worktrees/components/alert'
import type { LogLine } from '@/features/worktrees/running/components/job-panel'
import type { LogDispatch, LogView } from '@/features/worktrees/running/hooks/use-log-view'
import { clock, lineKey, onlyMatching, rangesIn, type ShownRow } from '@/features/worktrees/running/utils/log-search'
import { cn } from '@/lib/utils'

export function LogLines({
  lines,
  pattern,
  matches,
  current,
  view,
  dispatch,
  total,
  failure,
  loading,
}: {
  lines: LogLine[]
  pattern: RegExp | undefined
  matches: number[]
  current: number | undefined
  view: LogView
  dispatch: LogDispatch
  total: number | undefined
  failure: string | undefined
  loading: boolean
}) {
  // the line the view is on comes into sight once, as it becomes the one; following keeps the last line in sight
  const onCurrent = useCallback(
    (row: HTMLDivElement | null) => {
      if (row && current !== undefined) row.scrollIntoView({ block: 'center' })
    },
    [current],
  )
  const following = (box: HTMLDivElement | null) => {
    if (box && view.follow && current === undefined) box.scrollTop = box.scrollHeight
  }

  if (failure) return <Alert tone="red" className="mx-6" title="The lines could not be read" description={failure} />
  if (loading) return <p className="flex-1 px-6 py-2 text-xs text-muted-foreground">Reading its lines…</p>
  if (lines.length === 0) return <p className="flex-1 px-6 py-2 text-xs text-muted-foreground">No line yet.</p>

  const matched = new Set(matches)
  // only the lines matching: hidden counts between them, by their place among the lines held, or in the whole log
  const rows: ShownRow[] =
    view.only && pattern
      ? onlyMatching(
          matches.map((at) => (lines[at]?.n ?? at + 1) - 1),
          total ?? lines.length,
        ).map((row) => ('line' in row ? { line: matches[row.line] ?? 0 } : row))
      : lines.map((_, line) => ({ line }))
  let after = -1

  return (
    <div
      ref={following}
      role="log"
      aria-label="Its lines"
      className="min-h-0 flex-1 overflow-auto py-2"
      onScroll={(e) => {
        const box = e.currentTarget
        if (view.follow && box.scrollTop + box.clientHeight < box.scrollHeight - 8)
          dispatch({ type: 'follow', follow: false })
      }}
    >
      <div className="min-w-fit">
        {rows.map((row) => {
          if ('hidden' in row || 'since' in row) {
            // a gap is keyed by the line it follows: two may hide as many lines
            return (
              <p key={`gap-${after}`} className="px-6 font-mono text-xs leading-5 text-muted-foreground">
                {'hidden' in row ? `··· ${counted(row.hidden)} hidden` : `··· ${counted(row.since)} since`}
              </p>
            )
          }
          after = row.line
          const line = lines[row.line]
          if (!line) return null
          const on = row.line === current
          return (
            <LogRow
              key={lineKey(line, row.line)}
              line={line}
              // a mark after any line of the log (its number in the whole log, else its place among the lines held) ends
              // a run before it; the daemon gives no other sign, so a mark whose run before is gone reads Started
              first={(line.n ?? row.line + 1) === 1}
              on={on}
              onCurrent={on ? onCurrent : undefined}
              matched={matched.has(row.line)}
              pattern={pattern}
              stamps={view.stamps}
              wrap={view.wrap}
            />
          )
        })}
      </div>
    </div>
  )
}

// a line printed renders once: a job printing on re-renders only its new rows, keyed by the line, not its place
const LogRow = memo(function LogRow({
  line,
  first,
  on,
  onCurrent,
  matched,
  pattern,
  stamps,
  wrap,
}: {
  line: LogLine
  first: boolean
  on: boolean
  onCurrent: ((row: HTMLDivElement | null) => void) | undefined
  matched: boolean
  pattern: RegExp | undefined
  stamps: boolean
  wrap: boolean
}) {
  // the daemon's mark where a run begins: below a run before it, the job started again
  if (line.mark)
    return (
      <div
        ref={onCurrent}
        aria-current={on ? 'true' : undefined}
        className={cn('flex items-center gap-4 px-6 py-2 text-xs text-muted-foreground', on && 'bg-amber-500/20')}
      >
        <span className="h-px flex-1 bg-border" />
        {first ? 'Started' : 'Restarted'} {clock(line.at)}
        <span className="h-px flex-1 bg-border" />
      </div>
    )

  return (
    <div
      ref={onCurrent}
      aria-current={on ? 'true' : undefined}
      className={cn(
        'flex gap-4 px-6 font-mono text-xs leading-5',
        matched && 'bg-amber-500/8',
        on && 'bg-amber-500/20',
      )}
    >
      {stamps && <span className="w-16 shrink-0 text-muted-foreground tabular-nums">{clock(line.at)}</span>}
      <span className={cn('min-w-0', wrap ? 'break-all whitespace-pre-wrap' : 'whitespace-pre')}>
        <Marked text={line.text} pattern={pattern} current={on} />
      </span>
    </div>
  )
})

const counted = (count: number) => (count === 1 ? '1 line' : `${count} lines`)

function Marked({ text, pattern, current }: { text: string; pattern: RegExp | undefined; current: boolean }) {
  const ranges = pattern ? rangesIn(text, pattern) : []
  if (ranges.length === 0) return text

  const pieces = []
  let from = 0
  for (const [start, end] of ranges) {
    if (start > from) pieces.push(text.slice(from, start))
    pieces.push(
      <mark
        key={start}
        className={cn('rounded-sm text-foreground', current ? 'bg-amber-500 text-black' : 'bg-amber-500/40')}
      >
        {text.slice(start, end)}
      </mark>,
    )
    from = end
  }
  if (from < text.length) pieces.push(text.slice(from))
  return pieces
}
