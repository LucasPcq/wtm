import { CircleDot, CircleMinus, CircleX, LoaderCircle, type LucideIcon } from 'lucide-react'
import type { ListJob, RunState } from '@/api/types.gen'
import type { Tone } from '@/features/worktrees/components/glyph'
import { uptime } from '@/lib/time'

export const jobLook: Record<RunState, { icon: LucideIcon; tone: Tone }> = {
  running: { icon: CircleDot, tone: 'ok' },
  starting: { icon: LoaderCircle, tone: 'busy' },
  crashed: { icon: CircleX, tone: 'failed' },
  exited: { icon: CircleMinus, tone: 'muted' },
  stopped: { icon: CircleMinus, tone: 'muted' },
  not_started: { icon: CircleMinus, tone: 'muted' },
}

// shortcut: a crash says its exit code, not when it crashed, which list does not publish; the jobs read (LUC-345)
// brings it with each job's last line
export function jobSince(job: ListJob, now: number): string {
  if (job.state === 'crashed') return job.exit_code === undefined ? 'crashed' : `crashed · exit ${job.exit_code}`
  if (job.state === 'running' && job.started_at) return `up ${uptime(job.started_at, now)}`
  return job.state ?? ''
}
