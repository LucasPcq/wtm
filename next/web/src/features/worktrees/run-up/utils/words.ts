import type { Applied } from '@/api/queries/commands'
import type { DomainOption, RunupJobPlan, RunupJobStatus } from '@/api/types.gen'
import { named, type Shared } from '@/features/worktrees/run-up/utils/plan'
import type { PhaseRun } from '@/features/worktrees/utils/progress'

export const resultWords: Record<RunupJobStatus, string> = {
  started: 'up',
  already_running: 'already up',
  joined: 'joined',
  done: 'done',
  failed: 'failed',
  crashed: 'crashed',
  not_started: 'not started',
}

// a profile's jobs summed up to three and a count, as a menu row holds them
export const jobsSummary = (jobs: string[] = []) =>
  jobs.length > 3 ? `${jobs.slice(0, 3).join(', ')} +${jobs.length - 3}` : jobs.join(', ')

// what a worktree runs now, as the picker's second line: its own jobs, or the shared services it runs for all
export function runsNow({ up = [], crashed = 0, holds = [], profile }: DomainOption) {
  if (up.length === 0 && crashed === 0) return holds.length > 0 ? `holds ${named(holds)}` : 'nothing running'
  return [profile && `running ${profile}`, up.length > 0 && `${up.length} up`, crashed > 0 && `${crashed} crashed`]
    .filter(Boolean)
    .join(' · ')
}

export function profileLine({ jobs, default: isDefault, running_in }: DomainOption) {
  const runningIn = running_in?.length ? `already running in ${running_in.join(', ')}` : undefined
  return [jobsSummary(jobs), isDefault && 'the default', runningIn].filter(Boolean).join(' · ')
}

// what Confirm says a job does, from its plan: a port in mono, as the screen spells what a machine reads
export function planned(job: RunupJobPlan, first: string[]): { said: string; mono: boolean } {
  if (job.action === 'up') return { said: 'already up: left as it is', mono: false }
  if (job.kind === 'task')
    return { said: first.includes(job.name) ? 'a task: runs first, the others wait for it' : 'a task', mono: false }
  const ports = Object.values(job.ports ?? {})
  if (ports.length === 0) return { said: 'no address', mono: false }
  return { said: ports.map((port) => `:${port}`).join(' '), mono: true }
}

// a shared service as Confirm says it: up already in main or started there, then joined, with each its namespace
export function sharedLine({ holders, namespaces }: Shared, up: boolean) {
  const own = () => {
    if (namespaces.length === 0) return ''
    return holders > 1 ? ', a namespace each' : `, with ${namespaces[0]} of its own`
  }
  const who = () => {
    if (holders === 2) return 'both join it'
    return holders > 2 ? `the ${holders} join it` : 'joins it'
  }
  return `${holders > 1 ? '' : 'shared, '}${up ? 'already up' : 'starts in main'}: ${who()}${own()}`
}

// a shared service's row while the start runs, once for all the worktrees joining it
export const sharedSuffix = ({ holders, namespaces }: Shared) => {
  if (namespaces.length === 0) return ''
  return holders > 1 ? ' · a namespace for each' : ` · ${namespaces[0]}`
}

// a job under way, as its row says it; a step done says what it became once the outcome tells
export function underWay(run: PhaseRun, job: RunupJobPlan | undefined, cancelling: boolean): string {
  if (run.state === 'todo') {
    if (cancelling) return "won't start"
    const waits = job?.waits_for ?? []
    return waits.length > 0 ? `waits for ${waits.join(', ')}` : 'queued'
  }
  if (run.state === 'now') return job?.action === 'join' ? 'joining' : 'starting'
  if (run.state === 'failed') return 'failed'
  if (job?.action === 'join') return 'joined'
  if (job?.action === 'up') return 'already up'
  return job?.kind === 'task' ? 'done' : 'up'
}

export const where = (worktrees: string[]) =>
  worktrees.length === 1 ? `in ${worktrees[0]}` : `in ${worktrees.length} worktrees`

// what the start's jobs became, counted: the services up, the shared ones joined, the tasks done
export function startedWords(profile: string, envelope: Applied<'run-up'> | undefined) {
  const items = envelope?.items ?? []
  const jobs = items.flatMap((item) => item.detail?.jobs ?? [])
  const count = (statuses: RunupJobStatus[]) => jobs.filter((job) => statuses.includes(job.status)).length
  const said = [
    [count(['started', 'already_running']), 'up'],
    [count(['joined']), 'joined'],
    [count(['done']), 'done'],
  ] as const
  return {
    title: `${profile} started ${where(items.map((item) => item.subject))}`,
    description: said.flatMap(([n, word]) => (n > 0 ? [`${n} ${word}`] : [])).join(' · '),
  }
}
