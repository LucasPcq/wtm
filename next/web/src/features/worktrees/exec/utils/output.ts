import type { Unit } from '@/features/worktrees/utils/progress'

const esc = String.fromCharCode(27)
const bell = String.fromCharCode(7)
// CSI (colours, cursor moves), OSC (titles, links) and the short escapes (ESC 7, a charset's ESC ( B), as the core
// strips them from a tail
const escapes = new RegExp(
  `${esc}\\[[0-?]*[ -/]*[@-~]|${esc}\\][^${bell}${esc}]*(?:${bell}|${esc}\\\\)|${esc}[ -/]*[0-~]`,
  'g',
)

// a line as a terminal leaves it: its escapes dropped, and of the frames a \r redrew, the last
export function terminalLine(line: string): string {
  return (
    line
      .replace(escapes, '')
      .split('\r')
      .findLast((frame) => frame !== '') ?? ''
  ).trimEnd()
}

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
