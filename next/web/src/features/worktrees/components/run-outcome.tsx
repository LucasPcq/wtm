import { OutcomeLine } from '@/features/worktrees/components/outcome-line'
import { type OutcomeGroup, type OutcomeKind, outcomeOrder } from '@/features/worktrees/utils/outcome'

// a run's outcome in named groups; a lone group needs no name
export function RunOutcome({ groups, labels }: { groups: OutcomeGroup[]; labels: Record<OutcomeKind, string> }) {
  const shown = outcomeOrder.flatMap((kind) => groups.filter((group) => group.kind === kind && group.rows.length > 0))

  if (shown.length === 0) return null

  return (
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
  )
}
