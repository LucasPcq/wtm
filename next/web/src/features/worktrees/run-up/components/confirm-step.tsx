import { GitBranch, Link2 } from 'lucide-react'
import type { ReactNode } from 'react'
import type { DomainWarning, RunupPlan } from '@/api/types.gen'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { firstOf, howItRuns, numberedOf, sharedOf } from '@/features/worktrees/run-up/utils/plan'
import { planned, sharedLine } from '@/features/worktrees/run-up/utils/words'
import { planWide } from '@/features/worktrees/utils/outcome'
import { cn } from '@/lib/utils'

export function ConfirmStep({ plan, warnings }: { plan: RunupPlan; warnings: DomainWarning[] }) {
  const targets = plan.worktrees ?? []
  const shared = sharedOf(plan)
  const first = firstOf(plan)
  const several = targets.length > 1
  const sentences = howItRuns(plan)

  return (
    <>
      <PlanWarnings warnings={warnings.filter(planWide)} />
      <p className="flex items-center gap-2 text-sm">
        <GitBranch className="size-4 text-muted-foreground" aria-hidden="true" />
        <span className="font-medium">{several ? `${targets.length} worktrees` : targets[0]?.worktree}</span>
        <span className="text-muted-foreground">profile {plan.profile}</span>
      </p>

      {shared.length > 0 && (
        <Block title={several ? 'Shared services, joined once' : 'Shared services'}>
          {shared.map((service) => (
            <Line key={service.name} icon={<Link2 className="size-4 text-muted-foreground" />} name={service.name}>
              {sharedLine(service)}
            </Line>
          ))}
        </Block>
      )}

      {numberedOf(plan).map(({ worktree, jobs }) => (
        <Block key={worktree} title={several ? worktree : 'In this order'}>
          {jobs.map((job) => {
            const { said, mono } = planned(job, first)
            return (
              <Line key={job.name} icon={<Ordinal value={job.number} />} name={job.name} mono={mono}>
                {said}
              </Line>
            )
          })}
        </Block>
      ))}

      <Block title="How it runs">
        {sentences.map((sentence) => (
          <li key={sentence} className="ml-5 list-disc text-sm marker:text-muted-foreground">
            {sentence}
          </li>
        ))}
      </Block>
    </>
  )
}

function Block({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-xs font-medium text-muted-foreground">{title}</h3>
      <ul className="flex flex-col gap-2">{children}</ul>
    </section>
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
