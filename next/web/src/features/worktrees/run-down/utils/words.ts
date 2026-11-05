import type { RunFate, RunjobsStopJob, RunjobsStopStatus } from '@/api/types.gen'
import type { Shared } from '@/features/worktrees/run-down/utils/plan'
import type { PhaseRun } from '@/features/worktrees/utils/progress'

export const statusWords: Record<RunjobsStopStatus, string> = {
  stopped: 'stopped',
  released: 'released',
  cleared: 'cleared',
  not_running: 'not running',
  failed: "didn't stop",
  not_reached: 'not reached',
}

// past three names a line no longer holds: a count; none named, the plan saw nobody keeping it and another came since
export const kept = (worktrees: string[]) => {
  if (worktrees.length === 0) return 'another worktree'
  return worktrees.length <= 3 ? worktrees.join(', ') : `${worktrees.length} worktrees`
}

const who = (holders: string[]) => {
  if (holders.length === 1) return holders[0] ?? ''
  return holders.length === 2 ? 'both' : `the ${holders.length}`
}

const fateLine: Record<RunFate, (job: RunjobsStopJob) => { said: string; mono: boolean }> = {
  stop: (job) => (job.stop_command ? { said: job.stop_command, mono: true } : { said: 'stops', mono: false }),
  release: (job) => ({ said: `released: keeps running for ${kept(job.kept_by ?? [])}`, mono: false }),
  clear: () => ({ said: 'crashed: nothing to stop', mono: false }),
}

// what Confirm says one of a worktree's own jobs becomes: a stack's stop command in mono, as a machine reads it
export const confirmed = (job: RunjobsStopJob) => fateLine[job.fate](job)

export function sharedLine({ holders, keptBy, stops }: Shared) {
  const go = holders.length === 1 ? `${who(holders)} lets go` : `${who(holders)} let go`
  return stops ? `${go}: it stops, no worktree keeps it` : `${go}: keeps running for ${kept(keptBy)}`
}

// a job's row while the stop runs
export function stopping(run: PhaseRun, job: RunjobsStopJob | undefined) {
  if (job?.fate === 'clear') return 'crashed: nothing to stop'
  if (run.state === 'todo') return 'queued'
  if (run.state === 'now') return job?.stop_command ? 'running its stop command' : 'stopping'
  return run.state === 'failed' ? "didn't stop" : 'stopped'
}

// a shared service's row while the stop runs, once for all its holders: where the least advanced of them stands
// ended are its statuses once the outcome is known: what the stop did, over what the plan foresaw
export function sharedStopping(run: PhaseRun, { holders, keptBy, stops }: Shared, ended: RunjobsStopStatus[] = []) {
  const once = holders.length === 1 ? `${holders[0]} stops` : `${who(holders)} worktrees stop`
  if (run.state === 'todo') return `${stops ? 'stops' : 'released'} once ${once}`
  if (run.state === 'now') return stops ? 'stopping' : 'releasing'
  if (run.state === 'failed') return "didn't stop"
  if (ended.includes('stopped')) return 'stopped'
  if (ended.includes('released')) return `released: keeps running for ${kept(keptBy)}`
  return stops ? 'stopped' : `released: keeps running for ${kept(keptBy)}`
}

export const stopLabel = (worktrees: string[]) =>
  worktrees.length > 1 ? `Stop jobs in ${worktrees.length} worktrees` : 'Stop jobs'
