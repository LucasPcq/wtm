import type { DomainOption, RunjobsJobPlan } from '@/api/types.gen'
import type { PhaseRun } from '@/features/worktrees/utils/progress'

export const named = (names: string[]) =>
  names.length < 2 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`

// what a worktree runs now, as the picker's second line: its own jobs, or the shared services it runs for all
export function runsNow({ up = [], crashed = 0, holds = [], profile }: DomainOption) {
  if (up.length === 0 && crashed === 0) return holds.length > 0 ? `holds ${named(holds)}` : 'nothing running'
  return [profile && `running ${profile}`, up.length > 0 && `${up.length} up`, crashed > 0 && `${crashed} crashed`]
    .filter(Boolean)
    .join(' · ')
}

export const where = (worktrees: string[]) =>
  worktrees.length === 1 ? `in ${worktrees[0]}` : `in ${worktrees.length} worktrees`

// a profile's jobs summed up to three and a count, as a menu row holds them
export const jobsSummary = (jobs: string[] = []) =>
  jobs.length > 3 ? `${jobs.slice(0, 3).join(', ')} +${jobs.length - 3}` : jobs.join(', ')

// a job under way, as its row says it; a step done says what it became once the outcome tells
export function underWay(run: PhaseRun, job: RunjobsJobPlan | undefined, cancelling: boolean): string {
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
