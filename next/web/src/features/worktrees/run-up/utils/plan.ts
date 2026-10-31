import type { RunupJobPlan, RunupPlan } from '@/api/types.gen'
import { jobsTodo, type Unit } from '@/features/worktrees/utils/progress'

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

export type NumberedJob = RunupJobPlan & { number: number }

// the jobs a worktree runs itself, numbered across the run in the order they start
export function numberedOf(plan: RunupPlan): { worktree: string; jobs: NumberedJob[] }[] {
  const targets = plan.worktrees ?? []
  const own = targets.map((target) => (target.jobs ?? []).filter((job) => job.action !== 'join'))
  const offsets = own.map((_, index) => own.slice(0, index).reduce((count, jobs) => count + jobs.length, 0))
  return targets.map((target, index) => ({
    worktree: target.worktree,
    jobs: (own[index] ?? []).map((job, at) => ({ ...job, number: (offsets[index] ?? 0) + at + 1 })),
  }))
}

// the jobs the others wait for: the tasks that run first
export const firstOf = (plan: RunupPlan) => [
  ...new Set((plan.worktrees ?? []).flatMap((target) => (target.jobs ?? []).flatMap((job) => job.waits_for ?? []))),
]

const named = (names: string[]) =>
  names.length < 2 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`

export function howItRuns(plan: RunupPlan): string[] {
  const count = plan.worktrees?.length ?? 0
  const first = firstOf(plan)
  const spaced = sharedOf(plan).filter((shared) => shared.namespaces.length > 0)
  const others = (plan.others ?? []).map((other) => other.worktree)

  const together = () => {
    if (count < 2) return []
    const then = first.length > 0 ? `; in each, ${named(first)} ${first.length === 1 ? 'runs' : 'run'} first` : ''
    return [`The ${count} worktrees start side by side${then}.`]
  }

  return [
    ...(plan.exclusive && others.length > 0 ? [`The jobs of ${named(others)} stop first.`] : []),
    ...together(),
    ...spaced.map(({ name }) => `${count < 2 ? 'It gets' : 'Each gets'} a namespace of its own in the shared ${name}.`),
    'They keep running when this page closes; run down stops them.',
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
