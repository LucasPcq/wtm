import type { RunpsJob, RunState } from '@/api/types.gen'
import type { RunningGroup } from '@/features/worktrees/running/utils/groups'
import type { Chip, ShownState } from '@/features/worktrees/store/ui-store'
import { ago, uptime } from '@/lib/time'

export const stateWord: Record<RunState, string> = {
  starting: 'starting',
  running: 'running',
  crashed: 'crashed',
  exited: 'exited',
  stopped: 'stopped',
  not_started: 'not started',
}

export const chipLabel: Record<Chip, string> = { all: 'All', running: 'Running', crashed: 'Crashed' }

export const shownLabel: Record<ShownState, string> = {
  running: 'Running',
  crashed: 'Crashed',
  ended: 'Stopped and exited',
}

// a group's second line: its profile and its counts, the ended ones too once it is folded; the shared services say
// how many worktrees hold them
export function groupLine({ shared, profile, counts, starting }: RunningGroup, folded: boolean) {
  const ended: [number, string][] = [
    [counts.exited, 'exited'],
    [counts.stopped, 'stopped'],
    [counts.not_started, 'not started'],
  ]
  return [
    profile && `profile ${profile}`,
    counts.running > 0 && `${counts.running} up`,
    counts.crashed > 0 && `${counts.crashed} crashed`,
    starting && 'starting',
    ...(folded ? ended.map(([count, word]) => count > 0 && `${count} ${word}`) : []),
    shared && counts.holders && `held by ${counts.holders} ${counts.holders === 1 ? 'worktree' : 'worktrees'}`,
  ]
    .filter(Boolean)
    .join(' · ')
}

// the quiet row closing a group: "1 exited · migrate", or, of several states, "2 hidden · api stopped, migrate exited"
export function endedLine(ended: RunpsJob[]) {
  const states = new Set(ended.map((job) => job.state))
  const [only] = states
  if (states.size === 1 && only)
    return `${ended.length} ${stateWord[only]} · ${ended.map((job) => job.name).join(', ')}`
  return `${ended.length} hidden · ${ended.map((job) => `${job.name} ${stateWord[job.state]}`).join(', ')}`
}

// the quiet row once it shows them: "Exited jobs shown", "Ended jobs shown" of several states
export function endedShownLine(ended: RunpsJob[]) {
  const states = new Set(ended.map((job) => job.state))
  const [only] = states
  return states.size === 1 && only ? `${capital(stateWord[only])} jobs shown` : 'Ended jobs shown'
}

const capital = (word: string) => word.charAt(0).toUpperCase() + word.slice(1)

// where a job stands, with its time: "up 12 min", "crashed · exit 1 · 2 min ago", "exited 0 · 12 min ago"
export function jobSince(job: RunpsJob, now: number): string {
  const when = job.ended_at ? ago(job.ended_at, now) : undefined
  switch (job.state) {
    case 'running':
      return job.started_at ? `up ${uptime(job.started_at, now)}` : 'running'
    case 'crashed':
      return [job.exit_code === undefined ? 'crashed' : `crashed · exit ${job.exit_code}`, when]
        .filter(Boolean)
        .join(' · ')
    case 'exited':
      return [`exited ${job.exit_code ?? 0}`, when].filter(Boolean).join(' · ')
    case 'stopped':
      return [`stopped`, when].filter(Boolean).join(' · ')
    case 'starting':
    case 'not_started':
      return stateWord[job.state]
  }
}
