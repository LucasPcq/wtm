import { Link } from '@tanstack/react-router'
import { CircleDashed } from 'lucide-react'
import { toneIcon } from '@/features/worktrees/components/glyph'
import type { RunningRow } from '@/features/worktrees/running/utils/groups'
import { jobLook, jobSince } from '@/features/worktrees/running/utils/job-look'
import { withoutScheme } from '@/lib/url'
import { cn } from '@/lib/utils'

// a job the start under way brings up is greyed with where it stands until the read lists it
export function RunningJob({
  repo,
  worktree,
  row,
  now,
  selected,
}: {
  repo: string
  worktree: string
  row: RunningRow
  now: number
  selected: boolean
}) {
  if (!('job' in row))
    return (
      <li aria-label={row.name} className="flex items-center gap-3 py-1.5 text-sm text-muted-foreground">
        <CircleDashed className="size-4 shrink-0" aria-hidden="true" />
        <span className="flex-1 font-medium">{row.name}</span>
        <span className="text-xs">{row.said}</span>
      </li>
    )

  const { job } = row
  const { icon: Icon, tone } = jobLook[job.state ?? 'stopped']

  return (
    <li
      aria-label={job.name}
      className={cn(
        '-mx-2 flex flex-col gap-0.5 rounded-md px-2 py-1.5',
        selected ? 'bg-accent' : 'hover:bg-accent/50',
      )}
    >
      <Link
        to="/r/$repo/running"
        params={{ repo }}
        search={{ worktree, job: job.name }}
        aria-current={selected ? 'page' : undefined}
        className="flex items-center gap-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
        <span className={cn('flex-1 font-medium', tone === 'muted' && 'text-muted-foreground')}>{job.name}</span>
        <span className={cn('text-xs', tone === 'failed' ? 'text-destructive' : 'text-muted-foreground')}>
          {jobSince(job, now)}
        </span>
      </Link>
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
