import { Clock, GitBranch, Play, RotateCcw } from 'lucide-react'
import type { DomainWarning, RunstartPlan } from '@/api/types.gen'
import { JobLine } from '@/features/worktrees/components/job-line'
import { JobSection } from '@/features/worktrees/components/job-section'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { howItRuns, jobOf, plannedJob } from '@/features/worktrees/run-start/utils/words'
import { planWide } from '@/features/worktrees/utils/outcome'

export function ConfirmStep({ plan, warnings }: { plan: RunstartPlan; warnings: DomainWarning[] }) {
  const job = jobOf(plan)
  const planned = job && plannedJob(job)
  const Icon = job?.action === 'restart' ? RotateCcw : Play

  return (
    <>
      <PlanWarnings warnings={warnings.filter(planWide)} />
      <p className="flex items-center gap-2 text-sm">
        <GitBranch className="size-4 text-muted-foreground" aria-hidden="true" />
        <span className="font-medium">{plan.worktrees?.[0]?.worktree}</span>
      </p>

      {job && planned && (
        <JobSection title="What starts">
          <JobLine icon={<Icon className="size-4 text-muted-foreground" />} name={job.name}>
            {[planned.said, planned.ports].filter(Boolean).join(' · ')}
          </JobLine>
        </JobSection>
      )}

      <JobSection title="How it runs">
        {howItRuns(plan).map((text) => (
          <li key={text} className="flex items-start gap-2 text-sm">
            <Clock className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            {text}
          </li>
        ))}
      </JobSection>
    </>
  )
}
