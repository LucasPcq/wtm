import type { Applied } from '@/api/queries/commands'
import type { DomainOption, RunupJobPlan, RunupJobStatus } from '@/api/types.gen'
import type { OutcomeTone } from '@/features/worktrees/utils/outcome'
import type { PhaseRun } from '@/features/worktrees/utils/progress'

export const resultLook: Record<RunupJobStatus, { said: string; tone: OutcomeTone }> = {
  started: { said: 'up', tone: 'ok' },
  already_running: { said: 'already up', tone: 'ok' },
  joined: { said: 'joined', tone: 'ok' },
  done: { said: 'done', tone: 'ok' },
  failed: { said: 'failed', tone: 'failed' },
  crashed: { said: 'crashed', tone: 'failed' },
  not_started: { said: 'not started', tone: 'muted' },
}

// a profile's jobs summed up to three and a count, as a menu row holds them
export const jobsSummary = (jobs: string[] = []) =>
  jobs.length > 3 ? `${jobs.slice(0, 3).join(', ')} +${jobs.length - 3}` : jobs.join(', ')

// what a worktree runs now, as the picker's second line
export function runsNow({ up = [], profile }: Pick<DomainOption, 'up' | 'profile'>) {
  if (up.length === 0) return 'nothing running'
  return [profile && `running ${profile}`, `${up.length} up`].filter(Boolean).join(' · ')
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

export function sharedLine({ holders, namespaces }: { holders: number; namespaces: string[] }) {
  if (holders > 1)
    return `runs once in main: the ${holders} join it${namespaces.length > 0 ? ', a namespace each' : ''}`
  return `runs once in main: joins it${namespaces[0] ? `, with ${namespaces[0]} of its own` : ''}`
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

export function startedWords(profile: string, envelope: Applied<'run-up'> | undefined) {
  const items = envelope?.items ?? []
  const jobs = items.flatMap((item) => item.detail?.jobs ?? [])
  const up = jobs.filter((job) => job.status === 'started' || job.status === 'already_running').length
  return {
    title: `${profile} started ${where(items.map((item) => item.subject))}`,
    description: `${up} up${jobs.some((job) => job.status === 'joined') ? ' · shared services joined' : ''}`,
  }
}
