import type { Applied } from '@/api/queries/commands'
import type { DomainWarning, RunupJobStatus } from '@/api/types.gen'
import { resultWords } from '@/features/worktrees/run-up/utils/words'
import {
  itemWarnings,
  type OutcomeGroup,
  type OutcomeKind,
  type OutcomeRow,
  planWide,
} from '@/features/worktrees/utils/outcome'

type RunUpItem = Applied<'run-up'>['items'][number]

// a worktree's jobs said by what became of them, in the order the screen reads them: up, joined, done
const order: RunupJobStatus[] = ['failed', 'crashed', 'started', 'already_running', 'joined', 'done', 'not_started']

export const jobsSaid = (item: RunUpItem) => {
  const jobs = item.detail?.jobs ?? []
  return order
    .flatMap((status) => {
      const names = jobs.filter((job) => job.status === status).map((job) => job.name)
      return names.length > 0 ? [`${names.join(', ')} ${resultWords[status]}`] : []
    })
    .join(' · ')
}

const rowOf = (item: RunUpItem): OutcomeRow => {
  const base = { key: item.subject, title: item.subject, path: item.detail?.path }
  if (item.status === 'done') return { ...base, tone: 'ok', message: jobsSaid(item) }
  if (item.status === 'cancelled') return { ...base, tone: 'muted', message: `cancelled · ${jobsSaid(item)}` }
  const failed = item.detail?.jobs?.find((job) => job.status === 'failed' || job.status === 'crashed')
  const output = failed?.lines?.length ? { lines: failed.lines, log: failed.log } : undefined
  return { ...base, tone: 'failed', message: item.error?.message ?? jobsSaid(item), output }
}

export function recapOf(envelope: Applied<'run-up'> | undefined): {
  groups: OutcomeGroup[]
  labels: Record<OutcomeKind, string>
  warnings: DomainWarning[]
} {
  const items = envelope?.items ?? []
  const warnings = envelope?.warnings ?? []
  const subjects = warnings
    .filter((warning) => !planWide(warning))
    .map((warning) => ({ subject: warning.params?.worktree ?? '', warnings: [warning] }))

  return {
    groups: [
      { kind: 'failed', rows: items.filter((item) => item.status !== 'done').map(rowOf) },
      { kind: 'check', rows: itemWarnings([...subjects, ...items]) },
      { kind: 'done', rows: items.filter((item) => item.status === 'done').map(rowOf) },
    ],
    labels: { failed: "Didn't start", check: 'To check', done: 'Started' },
    warnings: warnings.filter(planWide),
  }
}
