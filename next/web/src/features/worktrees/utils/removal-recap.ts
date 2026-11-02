import type { DomainWarning } from '@/api/types.gen'
import { droppedWords } from '@/features/worktrees/utils/data-words'
import {
  itemWarnings,
  type OutcomeGroup,
  type OutcomeRow,
  outputOf,
  stoppedMessage,
} from '@/features/worktrees/utils/outcome'
import { owedOf, owedRow } from '@/features/worktrees/utils/owed'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

export type Teardown = 'clean' | 'prune'

// what became of each worktree's data is no thing to act on, so it goes with the steps rather than in "To check"; a
// drop that failed stays there
export function recapOf(
  progress: Progress<Teardown>,
  command: Teardown,
): {
  groups: OutcomeGroup[]
  warnings: DomainWarning[]
  data: string[]
} {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const plan = envelope?.plan
  const warnings = envelope?.warnings ?? []
  const said = [`${command}.data_kept`, `${command}.data_deferred`]
  // the drops owed the run planned to pay are items of their own, each said from what became of it
  const owed = items.filter((item) => owedOf(item))
  const worktrees = items.filter((item) => !owedOf(item))
  const unitOf = (subject: string) => progress.units.find((unit) => unit.subject === subject)

  const moved = (subject: string) =>
    plan?.reparent_children
      ? (plan.moves ?? [])
          .filter((move) => move.from === subject)
          .map((move) => `${move.branch} moved onto ${move.to}`)
          .join('; ')
      : ''

  return {
    groups: [
      {
        kind: 'failed',
        rows: worktrees
          .filter((item) => item.status === 'failed' || item.status === 'cancelled')
          .map(
            (item): OutcomeRow => ({
              key: item.subject,
              tone: item.status === 'cancelled' ? ('muted' as const) : ('failed' as const),
              title: item.subject,
              message:
                item.status === 'cancelled'
                  ? stoppedMessage(unitOf(item.subject))
                  : item.error && sentence(item.error.message),
              output: outputOf(unitOf(item.subject)),
              followUp: item.error?.follow_up,
            }),
          )
          .concat(owed.filter((item) => item.status !== 'done').map(owedRow)),
      },
      {
        kind: 'check',
        rows: itemWarnings(
          items.map((item) => ({
            subject: item.subject,
            warnings: (item.warnings ?? []).filter((warning) => !said.includes(warning.code)),
          })),
        ),
      },
      {
        kind: 'done',
        rows: worktrees
          .filter((item) => item.status === 'done')
          .map(
            (item): OutcomeRow => ({
              key: item.subject,
              tone: 'ok',
              title: item.subject,
              message: moved(item.subject) || undefined,
            }),
          )
          .concat(owed.filter((item) => item.status === 'done').map(owedRow)),
      },
    ],
    warnings,
    data: items.flatMap((item) =>
      (item.detail?.namespaces ?? []).flatMap((dropped) => {
        const fate = droppedWords[dropped.status]
        return fate ? [`${item.subject}: ${dropped.name} in ${dropped.job} ${fate(dropped)}`] : []
      }),
    ),
  }
}
