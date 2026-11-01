import type { RunupJobPlan, RunupPlan } from '@/api/types.gen'
import { jobsTodo, type PhaseRun, type Unit } from '@/features/worktrees/utils/progress'

export type Shared = { name: string; holders: number; namespaces: string[] }

// a shared service is run once in the main checkout: each worktree joins it, so it is said once for all
export function sharedOf(plan: RunupPlan): Shared[] {
  const shared = new Map<string, Shared>()
  for (const target of plan.worktrees ?? [])
    for (const job of target.jobs ?? []) {
      if (job.action !== 'join') continue
      const seen = shared.get(job.name) ?? { name: job.name, holders: 0, namespaces: [] }
      const namespaces = job.namespace ? [...seen.namespaces, job.namespace] : seen.namespaces
      shared.set(job.name, { name: job.name, holders: seen.holders + 1, namespaces })
    }
  return [...shared.values()]
}

export type OrderedJob = RunupJobPlan & { number: number | undefined }

// the jobs a worktree runs itself, in the order they start; those that start are numbered across the run, those up
// already are not
export function orderedOf(plan: RunupPlan): { worktree: string; jobs: OrderedJob[] }[] {
  const targets = plan.worktrees ?? []
  const own = targets.map((target) => (target.jobs ?? []).filter((job) => job.action !== 'join'))
  const starting = own.map((jobs) => jobs.filter((job) => job.action === 'start').length)
  const offsets = starting.map((_, index) => starting.slice(0, index).reduce((count, n) => count + n, 0))
  return targets.map((target, index) => {
    const jobs = own[index] ?? []
    return {
      worktree: target.worktree,
      jobs: jobs.map((job) => {
        const before = jobs.slice(0, jobs.indexOf(job) + 1).filter((one) => one.action === 'start').length
        return { ...job, number: job.action === 'start' ? (offsets[index] ?? 0) + before : undefined }
      }),
    }
  })
}

// the jobs the others wait for: the tasks that run first
export const firstOf = (plan: RunupPlan) => [
  ...new Set((plan.worktrees ?? []).flatMap((target) => (target.jobs ?? []).flatMap((job) => job.waits_for ?? []))),
]

export const named = (names: string[]) =>
  names.length < 2 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`

export type HowItRuns = { kind: 'stops' | 'together' | 'namespace' | 'others' | 'keeps'; text: string }

// others are the worktrees outside the start whose jobs run now
export function howItRuns(plan: RunupPlan, others: string[]): HowItRuns[] {
  const count = plan.worktrees?.length ?? 0
  const first = firstOf(plan)
  const spaced = sharedOf(plan).filter((shared) => shared.namespaces.length > 0)
  const stopped = (plan.others ?? []).map((other) => other.worktree)

  const together = (): HowItRuns[] => {
    if (count < 2) return []
    const then = first.length > 0 ? `; in each, ${named(first)} ${first.length === 1 ? 'runs' : 'run'} first` : ''
    return [{ kind: 'together', text: `The ${count} worktrees start side by side${then}.` }]
  }

  const kept = (): HowItRuns[] => {
    if (plan.exclusive || others.length === 0) return []
    const whose = others.length === 1 ? others[0] : `${others.length} other worktrees`
    return [{ kind: 'others', text: `The jobs of ${whose} keep running.` }]
  }

  return [
    ...(plan.exclusive && stopped.length > 0
      ? [{ kind: 'stops' as const, text: `The jobs of ${named(stopped)} stop first.` }]
      : []),
    ...together(),
    ...spaced.map(({ name }) => ({
      kind: 'namespace' as const,
      text: `${count < 2 ? 'It gets' : 'Each gets'} a namespace of its own in the shared ${name}.`,
    })),
    ...kept(),
    { kind: 'keeps', text: 'They keep running when this page closes; Stop jobs ends them.' },
  ]
}

// one unit per worktree, one phase per job, joins included: the stream reports each
export const unitsOf = (plan: RunupPlan): Unit[] =>
  (plan.worktrees ?? []).map((target) =>
    jobsTodo(
      target.worktree,
      (target.jobs ?? []).map((job) => job.name),
    ),
  )

// what a start brings up, counted: each worktree's own jobs, and each shared service once, up once every worktree
// joined it
export function countOf(units: Unit[], plan: RunupPlan | undefined): { done: number; total: number } {
  const joined = (worktree: string, name: string) =>
    plan?.worktrees?.find((target) => target.worktree === worktree)?.jobs?.find((job) => job.name === name)?.action ===
    'join'
  const runs = units.flatMap((unit) => unit.phases.map((run) => ({ run, shared: joined(unit.subject, run.job ?? '') })))
  const own = runs.filter(({ shared }) => !shared).map(({ run }) => run)
  const shared = new Map<string, PhaseRun[]>()
  for (const { run, shared: joins } of runs)
    if (joins) shared.set(run.job ?? '', [...(shared.get(run.job ?? '') ?? []), run])
  const ended = (state: string) => state === 'done' || state === 'failed'
  return {
    done:
      own.filter((run) => ended(run.state)).length +
      [...shared.values()].filter((joins) => joins.every((run) => ended(run.state))).length,
    total: own.length + shared.size,
  }
}
