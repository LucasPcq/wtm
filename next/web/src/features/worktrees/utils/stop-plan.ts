import type { RundownPlan, RunjobsStopJob } from '@/api/types.gen'
import { jobsTodo, type PhaseRun, type Unit } from '@/features/worktrees/utils/progress'

// holders: the worktrees of the stop that let it go; keptBy: those that keep it, none when it stops
export type Shared = { name: string; holders: string[]; keptBy: string[]; stops: boolean }

const isShared = (job: RunjobsStopJob) => job.shared_in !== undefined && job.shared_in !== ''

// a shared service is let go by each worktree of the stop that holds it: it is said once for all
export function sharedOf(plan: RundownPlan | undefined): Shared[] {
  const shared = new Map<string, Shared>()
  for (const target of plan?.worktrees ?? [])
    for (const job of (target.jobs ?? []).filter(isShared)) {
      const seen = shared.get(job.name) ?? { name: job.name, holders: [], keptBy: job.kept_by ?? [], stops: false }
      shared.set(job.name, {
        ...seen,
        holders: [...seen.holders, target.worktree],
        stops: seen.stops || job.fate === 'stop',
      })
    }
  return [...shared.values()]
}

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
  const shared = sharedOf(plan).map(({ name }) =>
    runs.filter(({ run, job }) => job && isShared(job) && run.job === name),
  )
  return {
    done: own.filter(ended).length + shared.filter((holds) => holds.every(({ run }) => ended(run))).length,
    total: own.length + shared.length,
  }
}
