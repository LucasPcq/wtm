import { Link } from '@tanstack/react-router'
import { CircleDashed, LoaderCircle } from 'lucide-react'
import { toneIcon } from '@/features/worktrees/components/glyph'
import { useJobDoing } from '@/features/worktrees/running/hooks/use-job-doing'
import type { RunningRow } from '@/features/worktrees/running/utils/groups'
import { jobLook, jobSince } from '@/features/worktrees/running/utils/job-look'
import { doingWords } from '@/features/worktrees/running/utils/job-words'
import { withoutScheme } from '@/lib/url'
import { cn } from '@/lib/utils'

// a job the start under way brings up is greyed with where it stands until the read lists it, as one a direct
// restart, stop or release of it reaches
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
  const reached = useJobDoing(repo, worktree, row.name)

  if (!('job' in row))
    return (
      <li aria-label={row.name} className="flex items-center gap-3 py-1.5 text-sm text-muted-foreground">
        <CircleDashed className="size-4 shrink-0" aria-hidden="true" />
        <span className="flex-1 font-medium">{row.name}</span>
        <span className="text-xs">{row.said}</span>
      </li>
    )

  const { job } = row
  const { icon: Icon, tone } = reached ? { icon: LoaderCircle, tone: 'busy' as const } : jobLook[job.state ?? 'stopped']
  const said = () => (reached ? doingWords[reached] : jobSince(job, now))

  return (
    <li
      aria-label={job.name}
      className={cn(
        '-mx-2 flex flex-col gap-0.5 rounded-md px-2 py-1.5',
        selected ? 'bg-accent' : 'hover:bg-accent/50',
        reached && 'opacity-60',
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
          {said()}
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
