import type { RundownPlan, RunjobsStopJob } from '@/api/types.gen'
import { jobsTodo, type PhaseRun, type Unit } from '@/features/worktrees/utils/progress'

// holders: the worktrees of the stop that let it go; keptBy: those that keep it, none when it stops
export type Shared = { name: string; sharedIn: string; holders: string[]; keptBy: string[]; stops: boolean }

const isShared = (job: RunjobsStopJob) => job.shared_in !== undefined && job.shared_in !== ''

// a shared service is let go by each worktree of the stop that holds it: it is said once for all, by its main checkout,
// since two repositories may each run one of that name
export function sharedOf(plan: RundownPlan | undefined): Shared[] {
  const shared = new Map<string, Shared>()
  for (const target of plan?.worktrees ?? [])
    for (const job of (target.jobs ?? []).filter(isShared)) {
      const key = `${job.shared_in}:${job.name}`
      const seen = shared.get(key) ?? {
        name: job.name,
        sharedIn: job.shared_in ?? '',
        holders: [],
        keptBy: job.kept_by ?? [],
        stops: false,
      }
      shared.set(key, {
        ...seen,
        holders: [...seen.holders, target.worktree],
        stops: seen.stops || job.fate === 'stop',
      })
    }
  return [...shared.values()]
}

// whether the job of that name in that worktree is a hold on this shared service
export const holdsOn = (plan: RundownPlan | undefined, worktree: string, name: string | undefined, service: Shared) =>
  name === service.name && stopJob(plan, worktree, name)?.shared_in === service.sharedIn

export const ownOf = (jobs: RunjobsStopJob[] | null) => (jobs ?? []).filter((job) => !isShared(job))

export const stopJob = (plan: RundownPlan | undefined, worktree: string, name: string) =>
  plan?.worktrees?.find((target) => target.worktree === worktree)?.jobs?.find((job) => job.name === name)

// a worktree with nothing to stop reports nothing: it has no unit
export const unitsOf = (plan: RundownPlan): Unit[] =>
  (plan.worktrees ?? [])
    .filter((target) => (target.jobs ?? []).length > 0)
    .map((target) =>
      jobsTodo(
        target.worktree,
        (target.jobs ?? []).map((job) => job.name),
      ),
    )

const ended = (run: PhaseRun) => run.state === 'done' || run.state === 'failed'

// what a stop ends, counted: each worktree's own jobs, and each shared service once, let go by every holder
export function countOf(units: Unit[], plan: RundownPlan | undefined): { done: number; total: number } {
  const runs = units.flatMap((unit) =>
    unit.phases.map((run) => ({ run, job: stopJob(plan, unit.subject, run.job ?? '') })),
  )
  const own = runs.filter(({ job }) => !job || !isShared(job)).map(({ run }) => run)
  const shared = sharedOf(plan).map((service) =>
    units.flatMap((unit) => unit.phases.filter((run) => holdsOn(plan, unit.subject, run.job, service))),
  )
  return {
    done: own.filter(ended).length + shared.filter((holds) => holds.every(ended)).length,
    total: own.length + shared.length,
  }
}
