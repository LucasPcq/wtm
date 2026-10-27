import type { DomainWarning } from '@/api/types.gen'
import { OutcomeLine } from '@/features/worktrees/components/outcome-line'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { type OutcomeGroup, type OutcomeKind, outcomeOrder, planWide } from '@/features/worktrees/utils/outcome'
import { sentence } from '@/features/worktrees/utils/sentence'

// a warning naming no worktree stays the plan's Alert, as on the steps; one naming a worktree is a row to check
export function RunOutcome({
  groups,
  labels,
  warnings = [],
}: {
  groups: OutcomeGroup[]
  labels: Record<OutcomeKind, string>
  warnings?: DomainWarning[] | undefined
}) {
  const toCheck = warnings
    .filter((warning) => !planWide(warning))
    .map((warning) => ({
      key: `${warning.code}-${warning.message}`,
      tone: 'attention' as const,
      message: sentence(warning.message),
      followUp: warning.follow_up,
    }))
  const merged = groups.map((group) =>
    group.kind === 'check' ? { ...group, rows: [...toCheck, ...group.rows] } : group,
  )
  const shown = outcomeOrder.flatMap((kind) => merged.filter((group) => group.kind === kind && group.rows.length > 0))

  return (
    <>
      <PlanWarnings warnings={warnings.filter(planWide)} />
      {shown.length > 0 && (
        <div className="flex flex-col gap-6">
          {shown.map((group) => (
            <section key={group.kind} aria-label={labels[group.kind]} className="flex flex-col gap-1">
              {shown.length > 1 && <h3 className="text-xs font-medium text-muted-foreground">{labels[group.kind]}</h3>}
              <ul className="flex flex-col gap-1">
                {group.rows.map((row) => (
                  <OutcomeLine key={row.key} row={row} />
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}
    </>
  )
}
