import type { Applied } from '@/api/queries/commands'
import type { RunjobsAction, RunjobsJobPlan, RunstartPlan } from '@/api/types.gen'
import type { OutcomeGroup, OutcomeKind } from '@/features/worktrees/utils/outcome'
import { named } from '@/features/worktrees/utils/run-words'
import { sentence } from '@/features/worktrees/utils/sentence'

const actionWords: Record<RunjobsAction, (job: RunjobsJobPlan) => string> = {
  start: (job) => (job.kind === 'task' ? 'runs once, then ends' : 'starts'),
  up: () => 'runs already: left as it is',
  join: (job) => [`joins it in ${job.shared_in ?? 'the main checkout'}`, job.namespace].filter(Boolean).join(' · '),
  restart: () => 'stops, then starts again; its log goes on',
}

// what Confirm says the start does to its job: a port in mono, as the screen spells what a machine reads
export function plannedJob(job: RunjobsJobPlan): { said: string; ports: string } {
  const ports = Object.values(job.ports ?? {})
    .map((port) => `:${port}`)
    .join(' ')
  return { said: actionWords[job.action](job), ports }
}

export const jobOf = (plan: RunstartPlan | undefined) => plan?.worktrees?.[0]?.jobs?.[0]

export function howItRuns(plan: RunstartPlan): string[] {
  const stopped = (plan.others ?? []).map((other) => other.worktree)
  const held = plan.held_by ?? []
  return [
    plan.exclusive && stopped.length > 0 && `The jobs of ${named(stopped)} stop first.`,
    held.length > 0 &&
      `${plan.job} stops for a moment in ${named(held)}, then runs for them again, their namespaces kept.`,
    jobOf(plan)?.kind === 'task'
      ? 'It runs to its end, even once this page closes.'
      : 'It keeps running when this page closes.',
  ].filter((line): line is string => typeof line === 'string')
}

export const startLabel = ({ job, restart }: { job: string; restart: boolean }) =>
  restart ? `Restart ${job}` : `Start ${job}`

export const outcomeLabels: Record<OutcomeKind, string> = { failed: 'Didn’t start', check: 'To check', done: 'Started' }

// the one job's outcome: what failed with its cause and its last lines, else where it is up
export function recapOf(envelope: Applied<'run-start'> | undefined): OutcomeGroup[] {
  const item = envelope?.items[0]
  const result = item?.detail?.jobs?.[0]
  if (!item || !result) return []
  if (item.status === 'failed' || result.status === 'failed' || result.status === 'crashed')
    return [
      {
        kind: 'failed',
        rows: [
          {
            key: result.name,
            tone: 'failed',
            title: result.name,
            message: item.error ? sentence(item.error.message) : result.status,
            output: result.lines?.length ? { lines: result.lines, log: result.log } : undefined,
          },
        ],
      },
    ]
  const port = result.ports?.[0]?.port
  const message = () => {
    if (result.status === 'done') return 'ran, then ended'
    if (result.status === 'joined') return 'joined'
    if (result.status === 'already_running') return 'runs already'
    return port === undefined ? 'up' : `up on :${port}`
  }
  return [{ kind: 'done', rows: [{ key: result.name, tone: 'ok', title: result.name, message: message() }] }]
}
