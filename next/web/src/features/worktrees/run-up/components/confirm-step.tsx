import { Clock, Database, GitBranch, Layers, Link2, type LucideIcon, Square, SquareStop } from 'lucide-react'
import type { ReactNode } from 'react'
import type { DomainOption, DomainWarning, RunupPlan } from '@/api/types.gen'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { JobSection } from '@/features/worktrees/run-up/components/job-section'
import {
  firstOf,
  type HowItRuns,
  howItRuns,
  orderedOf,
  type Shared,
  sharedOf,
} from '@/features/worktrees/run-up/utils/plan'
import { planned, sharedLine } from '@/features/worktrees/run-up/utils/words'
import { planWide } from '@/features/worktrees/utils/outcome'
import { cn } from '@/lib/utils'

const howIcon: Record<HowItRuns['kind'], LucideIcon> = {
  stops: SquareStop,
  together: Layers,
  namespace: Database,
  others: Square,
  keeps: Clock,
}

// options are the worktrees that start, holds the shared services up in the main checkout, others the worktrees
// outside the start whose jobs run now; a plan is none when every worktree chosen runs the profile already
export function ConfirmStep({
  plan,
  warnings,
  options,
  holds,
  others,
  left,
}: {
  plan: RunupPlan | undefined
  warnings: DomainWarning[]
  options: DomainOption[]
  holds: string[]
  others: string[]
  left: Left[]
}) {
  const leftRows = left.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)
  if (!plan) return <ItemList left={leftRows}>{[]}</ItemList>

  const targets = plan.worktrees ?? []
  const shared = sharedOf(plan)
  const first = firstOf(plan)
  const several = targets.length > 1
  const idle = options.every(({ up, crashed }) => (up?.length ?? 0) === 0 && !crashed)

  const sharedLines = shared.map((service: Shared) => (
    <Line key={service.name} icon={<Link2 className="size-4 text-muted-foreground" />} name={service.name}>
      {sharedLine(service, holds.includes(service.name))}
    </Line>
  ))

  return (
    <>
      <PlanWarnings warnings={warnings.filter(planWide)} />
      <p className="flex items-center gap-2 text-sm">
        <GitBranch className="size-4 text-muted-foreground" aria-hidden="true" />
        <span className="font-medium">{several ? `${targets.length} worktrees` : targets[0]?.worktree}</span>
        <span className="text-muted-foreground">
          {[plan.profile && `profile ${plan.profile}`, idle && 'nothing running there yet'].filter(Boolean).join(' · ')}
        </span>
      </p>

      {several && shared.length > 0 && <JobSection title="Shared services, joined once">{sharedLines}</JobSection>}

      {orderedOf(plan).map(({ worktree, jobs }) => (
        <JobSection key={worktree} title={several ? worktree : 'In this order'}>
          {!several && sharedLines}
          {jobs.map((job) => {
            const { said, mono } = planned(job, first)
            return (
              <Line key={job.name} icon={job.number && <Ordinal value={job.number} />} name={job.name} mono={mono}>
                {said}
              </Line>
            )
          })}
        </JobSection>
      ))}

      <ItemList left={leftRows}>{[]}</ItemList>

      <JobSection title="How it runs">
        {howItRuns(plan, others).map(({ kind, text }) => {
          const Icon = howIcon[kind]
          return (
            <li key={text} className="flex items-start gap-2 text-sm">
              <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              {text}
            </li>
          )
        })}
      </JobSection>
    </>
  )
}

function Line({
  icon,
  name,
  mono = false,
  children,
}: {
  icon: ReactNode
  name: string
  mono?: boolean
  children: ReactNode
}) {
  return (
    <li aria-label={name} className="flex items-center gap-3 text-sm">
      <span className="flex size-5 shrink-0 items-center justify-center">{icon}</span>
      <span className="font-medium">{name}</span>
      <span className={cn('min-w-0 truncate text-muted-foreground', mono && 'font-mono text-xs')}>{children}</span>
    </li>
  )
}

function Ordinal({ value }: { value: number }) {
  return (
    <span className="flex size-5 items-center justify-center rounded-full border text-[11px] text-muted-foreground">
      {value}
    </span>
  )
}
