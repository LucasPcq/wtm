import { CircleDashed, CircleDot, CircleMinus, CircleX, LoaderCircle, type LucideIcon } from 'lucide-react'
import type { RunState } from '@/api/types.gen'
import { type Tone, toneIcon } from '@/features/worktrees/components/glyph'
import type { RunningRow } from '@/features/worktrees/running/utils/groups'
import { uptime } from '@/lib/time'
import { withoutScheme } from '@/lib/url'
import { cn } from '@/lib/utils'

const look: Record<RunState, { icon: LucideIcon; tone: Tone }> = {
  running: { icon: CircleDot, tone: 'ok' },
  starting: { icon: LoaderCircle, tone: 'busy' },
  crashed: { icon: CircleX, tone: 'failed' },
  exited: { icon: CircleMinus, tone: 'muted' },
  stopped: { icon: CircleMinus, tone: 'muted' },
}

// a job the start under way brings up is greyed with where it stands until the read lists it
export function RunningJob({ row, now }: { row: RunningRow; now: number }) {
  if (!('job' in row))
    return (
      <li aria-label={row.name} className="flex items-center gap-3 py-1.5 text-sm text-muted-foreground">
        <CircleDashed className="size-4 shrink-0" aria-hidden="true" />
        <span className="flex-1 font-medium">{row.name}</span>
        <span className="text-xs">{row.said}</span>
      </li>
    )

  const { job } = row
  const { icon: Icon, tone } = look[job.state ?? 'stopped']

  // shortcut: a crash says its exit code, not when it crashed, which list does not publish; the jobs read (LUC-345)
  // brings it with each job's last line
  const since = () => {
    if (job.state === 'crashed') return job.exit_code === undefined ? 'crashed' : `crashed · exit ${job.exit_code}`
    if (job.state === 'running' && job.started_at) return `up ${uptime(job.started_at, now)}`
    return job.state ?? ''
  }

  return (
    <li aria-label={job.name} className="flex flex-col gap-0.5 py-1.5">
      <div className="flex items-center gap-3 text-sm">
        <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
        <span className={cn('flex-1 font-medium', tone === 'muted' && 'text-muted-foreground')}>{job.name}</span>
        <span className={cn('text-xs', tone === 'failed' ? 'text-destructive' : 'text-muted-foreground')}>
          {since()}
        </span>
      </div>
      {job.url && job.state === 'running' && (
        <a
          href={job.url}
          target="_blank"
          rel="noreferrer"
          className="ml-7 truncate font-mono text-xs text-muted-foreground underline-offset-4 hover:underline"
        >
          {withoutScheme(job.url)}
        </a>
      )}
    </li>
  )
}
