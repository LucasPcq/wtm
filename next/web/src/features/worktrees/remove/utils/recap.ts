import type { DomainWarning } from '@/api/types.gen'
import { sharedWarnings } from '@/features/worktrees/remove/utils/words'
import { type OutcomeGroup, outputOf } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

// what a removal that ended did, in the groups the dialog shows; the data each worktree keeps is no thing to act on,
// so it goes with the steps rather than in "To check"
export function recapOf(progress: Progress<'clean'>): { groups: OutcomeGroup[]; kept: string[] } {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const plan = envelope?.plan
  const warnings: DomainWarning[] = [...(envelope?.warnings ?? []), ...items.flatMap((item) => item.warnings ?? [])]
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
          .filter((item) => item.status === 'failed')
          .map((item) => ({
            key: item.subject,
            tone: 'failed' as const,
            title: item.subject,
            message: item.error && sentence(item.error.message),
            output: outputOf(unitOf(item.subject)),
            followUp: item.error?.follow_up,
          })),
      },
      {
        kind: 'check',
        rows: warnings
          .filter((warning) => warning.code !== sharedWarnings.dataKept)
          .map((warning) => ({
            key: `${warning.code}-${warning.message}`,
            tone: 'attention' as const,
            message: sentence(warning.message),
            followUp: warning.follow_up,
          })),
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
    kept: warnings
      .filter((warning) => warning.code === sharedWarnings.dataKept)
      .map((warning) => sentence(warning.message)),
  }
}
