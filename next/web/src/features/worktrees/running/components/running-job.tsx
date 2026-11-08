import { Link } from '@tanstack/react-router'
import { CircleDashed, GitBranch, LoaderCircle, Pin } from 'lucide-react'
import type { RunpsJob, RunState } from '@/api/types.gen'
import { toneIcon } from '@/features/worktrees/components/glyph'
import { Rail } from '@/features/worktrees/components/rail'
import { useJobDoing } from '@/features/worktrees/running/hooks/use-job-doing'
import type { RunningRow } from '@/features/worktrees/running/utils/groups'
import { jobLook } from '@/features/worktrees/running/utils/job-look'
import { doingWords } from '@/features/worktrees/running/utils/job-words'
import { jobSince } from '@/features/worktrees/running/utils/list-words'
import { withoutScheme } from '@/lib/url'
import { cn } from '@/lib/utils'

// a job the start under way brings up is greyed with where it stands until the read lists it, as one a direct
// restart, stop or release of it reaches; a pinned one says its worktree, as it sits out of its group
export function RunningJob({
  repo,
  worktree,
  row,
  now,
  selected,
  pinned = false,
}: {
  repo: string
  worktree: string
  row: RunningRow
  now: number
  selected: boolean
  pinned?: boolean
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
  const { icon: Icon, tone } = reached ? { icon: LoaderCircle, tone: 'busy' as const } : jobLook[job.state]
  const said = () => {
    if (reached) return doingWords[reached]
    const since =
      job.hold === 'detached' && job.state === 'running' ? `detached · ${jobSince(job, now)}` : jobSince(job, now)
    return pinned ? `${since} · ${worktree}` : since
  }
  const address = job.url && job.state === 'running' ? withoutScheme(job.url) : job.address

  return (
    <li aria-label={job.name} className={cn('flex flex-col', reached && 'opacity-60')}>
      <div
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
          <span
            className={cn(
              'flex min-w-0 flex-1 items-center gap-1.5 font-medium',
              tone === 'muted' && 'text-muted-foreground',
            )}
          >
            <span className="truncate">{job.name}</span>
            {pinned && <Pin className="size-3 shrink-0 text-muted-foreground" aria-label="pinned" />}
          </span>
          <span className={cn('shrink-0 text-xs', tone === 'failed' ? 'text-destructive' : 'text-muted-foreground')}>
            {said()}
          </span>
        </Link>
        {address && <span className="ml-7 truncate font-mono text-xs text-muted-foreground">{address}</span>}
        {job.last_line && (
          <span
            className={cn(
              'ml-7 truncate font-mono text-xs',
              job.state === 'crashed' ? 'text-destructive' : 'text-muted-foreground',
            )}
          >
            {job.last_line.text}
          </span>
        )}
      </div>
      <Children job={job} />
    </li>
  )
}

// under a runner the services it starts, under a shared service the worktrees holding it, with the namespace each got
function Children({ job }: { job: RunpsJob }) {
  if (job.services?.length)
    return (
      <Rail elbow={12} indent={14} className="ml-2">
        {job.services.map((service) => (
          <li key={service.name} className="flex items-center gap-2 py-1 text-[13px]">
            <ServiceGlyph state={service.state} />
            <span className="font-medium">{service.name}</span>
            {service.address && <span className="font-mono text-xs text-muted-foreground">{service.address}</span>}
          </li>
        ))}
      </Rail>
    )

  if (job.holders?.length)
    return (
      <Rail elbow={12} indent={14} className="ml-2">
        {job.holders.map((holder) => (
          <li key={holder.path} className="flex items-center gap-2 py-1 text-[13px]">
            <GitBranch className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="font-medium">{holder.branch || holder.path}</span>
            {holder.namespace && <span className="font-mono text-xs text-muted-foreground">{holder.namespace}</span>}
          </li>
        ))}
      </Rail>
    )

  return null
}

function ServiceGlyph({ state }: { state: RunState }) {
  const { icon: Icon, tone } = jobLook[state]
  return <Icon className={cn(toneIcon({ tone }), 'size-3.5 shrink-0')} aria-hidden="true" />
}
