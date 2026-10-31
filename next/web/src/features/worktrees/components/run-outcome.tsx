import type { DomainWarning } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { OutcomeLine } from '@/features/worktrees/components/outcome-line'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import {
  type Act,
  type OutcomeGroup,
  type OutcomeLabels,
  outcomeOrder,
  planWide,
} from '@/features/worktrees/utils/outcome'
import { sentence } from '@/features/worktrees/utils/sentence'

// a warning is drawn where its subject is: none, the plan's Alert, as on the steps; a worktree or branch, a row to check
export function RunOutcome({
  groups,
  labels,
  warnings = [],
  act,
}: {
  groups: OutcomeGroup[]
  labels: OutcomeLabels
  warnings?: DomainWarning[] | undefined
  act?: Act | undefined
}) {
  const toCheck = warnings
    .filter((warning) => !planWide(warning))
    .map((warning) => {
      const followUp = warning.follow_up
      return {
        key: `${warning.code}-${warning.message}`,
        tone: 'attention' as const,
        message: sentence(warning.message),
        followUp,
        act: act && followUp?.command === act.command ? () => act.run([followUp]) : undefined,
      }
    })
  const acted = toCheck.flatMap((row) => (row.act && row.followUp ? [row.followUp] : []))
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
              {group.kind === 'check' && act && acted.length > 1 ? (
                <div className="flex items-center justify-between gap-3">
                  <h3 className="text-xs font-medium text-muted-foreground">{labels[group.kind]}</h3>
                  <Button variant="outline" size="sm" disabled={act.busy} onClick={() => act.run(acted)}>
                    {act.all}
                  </Button>
                </div>
              ) : (
                shown.length > 1 && <h3 className="text-xs font-medium text-muted-foreground">{labels[group.kind]}</h3>
              )}
              <ul className="flex flex-col gap-1">
                {group.rows.map((row) => (
                  <OutcomeLine key={row.key} row={row} label={act?.label} busy={act?.busy ?? false} />
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}
    </>
  )
}
