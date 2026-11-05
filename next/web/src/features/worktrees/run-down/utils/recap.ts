import type { Applied } from '@/api/queries/commands'
import type { DomainWarning, RundownPlan, RunjobsStopStatus } from '@/api/types.gen'
import { sharedOf, stopJob } from '@/features/worktrees/run-down/utils/plan'
import { kept, statusWords } from '@/features/worktrees/run-down/utils/words'
import {
  itemWarnings,
  type OutcomeGroup,
  type OutcomeKind,
  type OutcomeRow,
  planWide,
} from '@/features/worktrees/utils/outcome'
import type { Unit } from '@/features/worktrees/utils/progress'

type RunDownItem = Applied<'run-down'>['items'][number]

// a worktree's own jobs said by what became of them: what went wrong first, what stopped, then the rest
const order: RunjobsStopStatus[] = ['failed', 'not_reached', 'stopped', 'cleared', 'not_running']

// units are the stop's progress: a worktree that failed shows the lines of the jobs that would not stop
export function recapOf(
  envelope: Applied<'run-down'> | undefined,
  plan: RundownPlan | undefined,
  units: Unit[] = [],
): { groups: OutcomeGroup[]; labels: Record<OutcomeKind, string>; warnings: DomainWarning[] } {
  const items = envelope?.items ?? []
  const warnings = envelope?.warnings ?? []
  const shared = sharedOf(plan)
  // by the plan's job of that worktree: a job of its own may be named like a shared service
  const isShared = (worktree: string, name: string) => Boolean(stopJob(plan, worktree, name)?.shared_in)

  // a worktree that held shared services alone says it let them go
  const said = (item: RunDownItem) => {
    const all = item.detail?.jobs ?? []
    const own = all.filter((job) => !isShared(item.subject, job.name))
    const letGo = all
      .filter((job) => isShared(item.subject, job.name) && (job.status === 'released' || job.status === 'stopped'))
      .map((job) => job.name)
    if (own.length === 0 && letGo.length > 0) return `let go of ${letGo.join(', ')}`
    // one that held shared services alone and was cut short says what of them was not reached
    const jobs = own.length > 0 ? own : all
    return order
      .flatMap((status) => {
        const names = jobs.filter((job) => job.status === status).map((job) => job.name)
        if (names.length === 0) return []
        return status === 'stopped' ? [names.join(', ')] : [`${names.join(', ')} ${statusWords[status]}`]
      })
      .join(' · ')
  }

  const rowOf = (item: RunDownItem): OutcomeRow => {
    const base = { key: item.subject, title: item.subject, path: item.detail?.path }
    if (item.status === 'done' && item.reason === 'unchanged')
      return { ...base, tone: 'muted', message: 'nothing was running' }
    if (item.status === 'done') return { ...base, tone: 'ok', message: said(item) }
    const lines = (units.find((unit) => unit.subject === item.subject)?.phases ?? [])
      .filter((run) => run.state === 'failed')
      .flatMap((run) => run.lines ?? [])
    const output = lines.length > 0 ? { lines } : undefined
    return { ...base, tone: 'failed', message: item.error?.message ?? said(item), output }
  }

  // a shared service says what its holders' stops did to it, once
  const services = shared.map((service) => {
    const statuses = items.flatMap((item) =>
      (item.detail?.jobs ?? []).filter((job) => job.name === service.name && isShared(item.subject, job.name)),
    )
    const status = (wanted: RunjobsStopStatus) => statuses.some((job) => job.status === wanted)
    const row = { key: `shared:${service.name}`, title: service.name }
    if (status('failed')) return { row: { ...row, tone: 'failed' as const, message: "didn't stop" }, group: 'failed' }
    if (status('stopped'))
      return { row: { ...row, tone: 'ok' as const, message: 'stopped: no worktree kept it' }, group: 'stopped' }
    if (status('released'))
      return {
        row: { ...row, tone: 'ok' as const, message: `keeps running for ${kept(service.keptBy)}` },
        group: 'released',
      }
    if (status('not_running'))
      return { row: { ...row, tone: 'muted' as const, message: 'already stopped' }, group: 'stopped' }
    if (status('not_reached'))
      return { row: { ...row, tone: 'muted' as const, message: 'not reached' }, group: 'failed' }
    return undefined
  })
  const servicesIn = (group: string) => services.flatMap((service) => (service?.group === group ? [service.row] : []))

  const subjects = warnings
    .filter((warning) => !planWide(warning))
    .map((warning) => ({ subject: warning.params?.worktree ?? '', warnings: [warning] }))

  return {
    groups: [
      { kind: 'failed', rows: [...items.filter((item) => item.status !== 'done').map(rowOf), ...servicesIn('failed')] },
      { kind: 'check', rows: itemWarnings([...subjects, ...items]) },
      { kind: 'done', label: 'Released', rows: servicesIn('released') },
      { kind: 'done', rows: [...items.filter((item) => item.status === 'done').map(rowOf), ...servicesIn('stopped')] },
    ],
    labels: { failed: "Couldn't stop", check: 'To check', done: 'Stopped' },
    warnings: warnings.filter(planWide),
  }
}
