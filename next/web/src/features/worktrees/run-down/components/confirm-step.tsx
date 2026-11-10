import { CircleDot, CircleX, Database, FileText, GitBranch, LogOut, type LucideIcon } from 'lucide-react'
import type { DomainOption, DomainWarning, RundownPlan, RundownTarget, RunFate } from '@/api/types.gen'
import { type Tone, toneIcon } from '@/features/worktrees/components/glyph'
import { ItemList } from '@/features/worktrees/components/item-list'
import { JobLine } from '@/features/worktrees/components/job-line'
import { JobSection } from '@/features/worktrees/components/job-section'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { planWide } from '@/features/worktrees/utils/outcome'
import { ownOf, sharedOf } from '@/features/worktrees/utils/stop-plan'
import { confirmed, sharedLine } from '@/features/worktrees/utils/stop-words'

const fateLook: Record<RunFate, { icon: LucideIcon; tone: Tone }> = {
  stop: { icon: CircleDot, tone: 'ok' },
  release: { icon: LogOut, tone: 'muted' },
  clear: { icon: CircleX, tone: 'failed' },
}

// options are the worktrees of the stop, each with what it runs now; a worktree with nothing to stop is left as it is
export function ConfirmStep({
  plan,
  warnings,
  options,
  left,
}: {
  plan: RundownPlan
  warnings: DomainWarning[]
  options: DomainOption[]
  left: Left[]
}) {
  const targets = plan.worktrees ?? []
  const stopping = targets.filter((target) => (target.jobs ?? []).length > 0)
  const shared = sharedOf(plan)
  const several = stopping.length > 1
  const up = options.reduce((count, option) => count + (option.up?.length ?? 0), 0)
  const crashed = options.reduce((count, option) => count + (option.crashed ?? 0), 0)
  const leftRows = [
    ...targets
      .filter((target) => !stopping.includes(target))
      .map((target) => ({ worktree: target.worktree, why: 'nothing running' })),
    ...left,
  ].map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)

  const counts = [up > 0 && `${up} up`, crashed > 0 && `${crashed} crashed`].filter(Boolean).join(', ')
  const title = (target: RundownTarget) => {
    if (several) return [target.worktree, target.profile && `profile ${target.profile}`].filter(Boolean).join(' · ')
    return target.profile ? `profile ${target.profile}` : 'Its jobs'
  }

  if (stopping.length === 0) return <ItemList left={leftRows}>{[]}</ItemList>

  return (
    <>
      <PlanWarnings warnings={warnings.filter(planWide)} />
      <p className="flex items-center gap-2 text-sm">
        <GitBranch className="size-4 text-muted-foreground" aria-hidden="true" />
        <span className="font-medium">{several ? `${stopping.length} worktrees` : stopping[0]?.worktree}</span>
        <span className="text-muted-foreground">
          {[several ? 'their jobs stop' : 'its jobs stop', counts].filter(Boolean).join(' · ')}
        </span>
      </p>

      {stopping.map((target) => {
        const own = ownOf(target.jobs)
        if (own.length === 0) return null
        return (
          <JobSection key={target.worktree} title={title(target)}>
            {own.map((job) => {
              const { icon: Icon, tone } = fateLook[job.fate]
              const { said, mono } = confirmed(job)
              return (
                <JobLine key={job.name} icon={<Icon className={toneIcon({ tone })} />} name={job.name} mono={mono}>
                  {said}
                </JobLine>
              )
            })}
          </JobSection>
        )
      })}

      {shared.length > 0 && (
        <JobSection title={shared.every((service) => !service.stops) ? 'Shared services, released' : 'Shared services'}>
          {shared.map((service) => (
            <JobLine key={service.name} icon={<LogOut className={toneIcon({ tone: 'muted' })} />} name={service.name}>
              {sharedLine(service)}
            </JobLine>
          ))}
        </JobSection>
      )}

      <ItemList left={leftRows}>{[]}</ItemList>

      <JobSection title="How it runs">
        {shared.length > 0 && (
          <How icon={Database}>{several ? 'Their' : 'Its'} databases stay: stopping drops nothing.</How>
        )}
        <How icon={FileText}>{several ? 'Their' : 'Its'} logs stay here; Start a profile brings the jobs back.</How>
      </JobSection>
    </>
  )
}

function How({ icon: Icon, children }: { icon: LucideIcon; children: string[] | string }) {
  return (
    <li className="flex items-start gap-2 text-sm">
      <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      {children}
    </li>
  )
}
