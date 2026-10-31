import type { Applied } from '@/api/queries/commands'
import type { DomainWarning } from '@/api/types.gen'
import { duration, terminalLine } from '@/features/worktrees/exec/utils/output'
import { warningLine } from '@/features/worktrees/exec/utils/words'
import {
  itemWarnings,
  type OutcomeGroup,
  type OutcomeLabels,
  type OutcomeRow,
  planWide,
} from '@/features/worktrees/utils/outcome'

type ExecItem = Applied<'exec'>['items'][number]

const outputOf = (item: ExecItem) => {
  const lines = (item.detail?.tail ?? []).map(terminalLine)
  if (lines.length === 0 && !item.detail?.log) return undefined
  return { lines, log: item.detail?.log }
}

const rowOf = (item: ExecItem): OutcomeRow => {
  const took = item.detail ? duration(item.detail.duration_ms) : undefined
  const base = { key: item.subject, title: item.subject, output: outputOf(item) }

  if (item.status === 'done') return { ...base, tone: 'ok', message: took }
  if (item.status === 'skipped') return { ...base, tone: 'muted', message: 'not reached' }
  if (item.status === 'cancelled') return { ...base, tone: 'muted', message: 'cancelled while running' }
  const exit = item.detail?.exit_code
  if (exit === null || exit === undefined)
    return {
      ...base,
      tone: 'failed',
      message: "couldn't start",
      output: base.output ?? (item.error && { lines: [item.error.message] }),
    }
  return { ...base, tone: 'failed', message: [`exited ${exit}`, took].filter(Boolean).join(' · ') }
}

// a warning on a worktree is a row to check, in the screen's words; the plan's own stay its Alert
export function recapOf(envelope: Applied<'exec'> | undefined): {
  groups: OutcomeGroup[]
  labels: OutcomeLabels
  warnings: DomainWarning[]
} {
  const items = envelope?.items ?? []
  const warnings = envelope?.warnings ?? []
  const failed = items.filter((item) => item.status === 'failed')
  const stopped = items.filter((item) => item.status === 'cancelled' || item.status === 'skipped')
  // a run warning about one worktree reads as that worktree's own
  const subjects = warnings
    .filter((warning) => !planWide(warning))
    .map((warning) => ({ subject: warning.params?.worktree ?? warning.params?.branch ?? '', warnings: [warning] }))

  const failedLabel = () => {
    if (failed.length > 0 && stopped.length > 0) return "Didn't pass"
    return stopped.length > 0 ? 'Stopped' : 'Failed'
  }

  return {
    groups: [
      { kind: 'failed', rows: [...failed, ...stopped].map(rowOf) },
      { kind: 'check', rows: itemWarnings([...subjects, ...items], warningLine) },
      { kind: 'done', rows: items.filter((item) => item.status === 'done').map(rowOf) },
    ],
    labels: { failed: failedLabel(), check: 'To check', done: 'Passed' },
    warnings: warnings.filter(planWide),
  }
}
