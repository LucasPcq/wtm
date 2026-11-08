import { ChevronRight, Copy, ExternalLink, GitBranch, LoaderCircle, Pin, PinOff } from 'lucide-react'
import type { RunlogsJob, RunpsJob, RunState } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { toneIcon } from '@/features/worktrees/components/glyph'
import { JobActions } from '@/features/worktrees/running/components/job-actions'
import { JobMenu } from '@/features/worktrees/running/components/job-menu'
import { useJobActions } from '@/features/worktrees/running/hooks/use-job-actions'
import { jobLook } from '@/features/worktrees/running/utils/job-look'
import { doingWords } from '@/features/worktrees/running/utils/job-words'
import { jobSince } from '@/features/worktrees/running/utils/list-words'
import { pinOf } from '@/features/worktrees/running/utils/visible'
import { useMarks } from '@/features/worktrees/store/ui-store'
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
}: {
  repo: string
  worktree: string
  profile: string | undefined
  job: RunpsJob
  facts: RunlogsJob | undefined
  now: number
}) {
  const { acts, doing, shared, holders } = useJobActions({ repo, worktree, job })
  const look = jobLook[job.state]
  const pins = useMarks('pinned', repo)
  const pinned = pins.has(pinOf(worktree, job.name))
  const togglePin = () => pins.toggle(pinOf(worktree, job.name))
  const PinIcon = pinned ? PinOff : Pin
  const { icon: Icon, tone } = doing ? { icon: LoaderCircle, tone: 'busy' as const } : look

  // a shared service runs once for every worktree holding it, in the main checkout
  const forAll = (job.holders?.length ?? 0) > 0
  const since = () => {
    if (doing) return doingWords[doing]
    if (forAll) return `${jobSince(job, now)} · runs once, in ${worktree}’s checkout`
    return job.hold === 'detached' ? `detached · ${jobSince(job, now)}` : jobSince(job, now)
  }
  const where = [facts?.dir, facts?.command].filter(Boolean).join(' · ')

  return (
    <header className="flex flex-col gap-1 px-6 pt-5 pb-4">
      <nav aria-label="Where it runs" className="flex items-center gap-1 text-xs text-muted-foreground">
        <span className="truncate">{forAll ? 'Shared services' : worktree}</span>
        {profile && !forAll && (
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
            {since()}
          </span>
        </h2>
        <div className="flex shrink-0 items-center gap-1">
          <JobActions acts={acts} job={job.name} />
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={pinned ? `Unpin ${job.name}` : `Pin ${job.name} to the top`}
            aria-pressed={pinned}
            onClick={togglePin}
          >
            <PinIcon />
          </Button>
          <JobMenu
            pinned={pinned}
            onPin={togglePin}
            at={{ repo, worktree, job: job.name }}
            url={job.url}
            log={facts?.log}
            acts={acts}
            shared={shared}
            holders={holders}
          />
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
      {job.services?.length ? (
        <div className="flex flex-col gap-1 pt-2">
          <p className="text-[13px] font-medium">
            Runs {job.services.length === 1 ? '1 service' : `${job.services.length} services`}
          </p>
          {job.services.map((service) => (
            <p key={service.name} className="flex items-center gap-2 text-[13px]">
              <ServiceGlyph state={service.state} />
              <span className="font-medium">{service.name}</span>
              {service.address && <span className="font-mono text-xs text-muted-foreground">{service.address}</span>}
            </p>
          ))}
        </div>
      ) : null}
      {job.holders?.length ? (
        <div className="flex flex-col gap-1 pt-2">
          <p className="text-[13px] font-medium">
            Held by {job.holders.length === 1 ? '1 worktree' : `${job.holders.length} worktrees`}
            {job.holders.some((holder) => holder.namespace) && ', a namespace each'}
          </p>
          {job.holders.map((holder) => (
            <p key={holder.path} className="flex items-center gap-2 text-[13px]">
              <GitBranch className="size-4 text-muted-foreground" aria-hidden="true" />
              <span className="font-medium">{holder.branch || holder.path}</span>
              {holder.namespace && <span className="font-mono text-xs text-muted-foreground">{holder.namespace}</span>}
            </p>
          ))}
        </div>
      ) : null}
    </header>
  )
}

function ServiceGlyph({ state }: { state: RunState }) {
  const { icon: Icon, tone } = jobLook[state]
  return <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
}
