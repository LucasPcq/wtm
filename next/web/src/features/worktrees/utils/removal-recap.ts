import type { DomainWarning } from '@/api/types.gen'
import { itemWarnings, type OutcomeGroup, outputOf, stoppedMessage } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

export type Teardown = 'clean' | 'prune'

// the data each worktree keeps is no thing to act on, so it goes with the steps rather than in "To check"
export function recapOf(
  progress: Progress<Teardown>,
  dataKept: string,
): {
  groups: OutcomeGroup[]
  warnings: DomainWarning[]
  kept: string[]
} {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const plan = envelope?.plan
  const warnings = envelope?.warnings ?? []
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
        rows: items
          .filter((item) => item.status === 'failed' || item.status === 'cancelled')
          .map((item) => ({
            key: item.subject,
            tone: item.status === 'cancelled' ? ('muted' as const) : ('failed' as const),
            title: item.subject,
            message:
              item.status === 'cancelled'
                ? stoppedMessage(unitOf(item.subject))
                : item.error && sentence(item.error.message),
            output: outputOf(unitOf(item.subject)),
            followUp: item.error?.follow_up,
          })),
      },
      {
        kind: 'check',
        rows: itemWarnings(items),
      },
      {
        kind: 'done',
        rows: items
          .filter((item) => item.status === 'done')
          .map((item) => ({
            key: item.subject,
            tone: 'ok' as const,
            title: item.subject,
            message: moved(item.subject) || undefined,
          })),
      },
    ],
    warnings: warnings.filter((warning) => warning.code !== dataKept),
    kept: warnings.filter((warning) => warning.code === dataKept).map((warning) => sentence(warning.message)),
  }
}
