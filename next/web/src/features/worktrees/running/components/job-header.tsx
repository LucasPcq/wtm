import { ChevronRight, Copy, ExternalLink } from 'lucide-react'
import type { ReactNode } from 'react'
import type { ListJob, RunlogsJob } from '@/api/types.gen'
import { toneIcon } from '@/features/worktrees/components/glyph'
import { JobMenu } from '@/features/worktrees/running/components/job-menu'
import { jobLook, jobSince } from '@/features/worktrees/running/utils/job-look'
import { copy } from '@/features/worktrees/utils/copy'
import { withoutScheme } from '@/lib/url'
import { cn } from '@/lib/utils'

export function JobHeader({
  repo,
  worktree,
  profile,
  job,
  facts,
  now,
  actions,
}: {
  repo: string
  worktree: string
  profile: string | undefined
  job: ListJob
  facts: RunlogsJob | undefined
  now: number
  actions?: ReactNode
}) {
  const { icon: Icon, tone } = jobLook[job.state ?? 'stopped']
  const where = [facts?.dir || '.', facts?.command].filter(Boolean).join(' · ')

  return (
    <header className="flex flex-col gap-1 px-6 pt-5 pb-4">
      <nav aria-label="Where it runs" className="flex items-center gap-1 text-xs text-muted-foreground">
        <span className="truncate">{worktree}</span>
        {profile && (
          <>
            <ChevronRight className="size-3 shrink-0" aria-hidden="true" />
            <span className="truncate">profile {profile}</span>
          </>
        )}
      </nav>
      <div className="flex items-center gap-3">
        <h2 className="flex min-w-0 flex-1 items-center gap-2">
          <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
          <span className="truncate text-lg font-semibold">{job.name}</span>
          <span className={cn('shrink-0 text-sm', tone === 'failed' ? 'text-destructive' : 'text-muted-foreground')}>
            {job.hold === 'detached' ? `detached · ${jobSince(job, now)}` : jobSince(job, now)}
          </span>
        </h2>
        <div className="flex shrink-0 items-center gap-1">
          {actions}
          <JobMenu at={{ repo, worktree, job: job.name }} url={job.url} log={facts?.log} />
        </div>
      </div>
      {job.url && (
        <a
          href={job.url}
          target="_blank"
          rel="noreferrer"
          className="flex w-fit max-w-full items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground"
        >
          <span className="truncate">{withoutScheme(job.url)}</span>
          <ExternalLink className="size-3 shrink-0" aria-hidden="true" />
        </a>
      )}
      {facts?.command && (
        <button
          type="button"
          aria-label={`Copy ${facts.command}`}
          onClick={() => copy(facts.command ?? '')}
          className="flex w-fit max-w-full items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground"
        >
          <span className="truncate">{where}</span>
          <Copy className="size-3 shrink-0" aria-hidden="true" />
        </button>
      )}
    </header>
  )
}
