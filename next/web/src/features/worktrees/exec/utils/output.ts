import type { Unit } from '@/features/worktrees/utils/progress'

export function duration(ms: number): string {
  return `${(ms / 1000).toFixed(1)} s`
}

export function elapsed(since: string, now: number): string {
  return `${Math.max(0, Math.floor((now - Date.parse(since)) / 1000))} s`
}

// a worktree while the command runs: queued, its time under way, or how it ended
export function targetLabel(unit: Unit, now: number, cancelling: boolean): string {
  const run = unit.phases[0]
  const state = run?.state ?? 'todo'
  if (state === 'todo') return cancelling ? "won't start" : 'queued'
  if (state === 'now') return cancelling || !run?.startedAt ? 'stopping' : elapsed(run.startedAt, now)
  const took = run?.startedAt && run.endedAt ? duration(Date.parse(run.endedAt) - Date.parse(run.startedAt)) : ''
  return state === 'failed' ? ['failed', took].filter(Boolean).join(' · ') : took
}
