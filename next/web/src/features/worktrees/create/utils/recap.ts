import { itemWarnings, type OutcomeGroup, outputOf, stoppedMessage } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

// a next step no item claims is said plainly with what went: nothing failed, nothing to check
export function recapOf(progress: Progress<'create'>): OutcomeGroup[] {
  const envelope = progress.envelope
  if (!envelope) return []

  const { items, warnings, follow_ups: followUps } = envelope
  const unitOf = (subject: string) => progress.units.find((unit) => unit.subject === subject)
  const nextOf = (subject: string) =>
    followUps.find((followUp) =>
      followUp.answers?.some((answer) => answer.path === 'worktree' && answer.values[0] === subject),
    )
  const loose = followUps.filter((followUp) => !items.some((item) => nextOf(item.subject) === followUp))

  const problem = (item: (typeof items)[number]) => {
    const unit = unitOf(item.subject)
    const cancelled = item.status === 'cancelled'
    return {
      key: item.subject,
      tone: cancelled ? ('muted' as const) : ('failed' as const),
      title: item.subject,
      message: cancelled ? stoppedMessage(unit) : item.error && sentence(item.error.message),
      output: outputOf(unit),
      followUp: nextOf(item.subject),
    }
  }

  return [
    {
      kind: 'failed',
      rows: items.filter((item) => item.status === 'failed' || item.status === 'cancelled').map(problem),
    },
    {
      kind: 'check',
      rows: [
        ...warnings.map((warning) => ({
          key: `${warning.code}-${warning.message}`,
          tone: 'attention' as const,
          message: sentence(warning.message),
          followUp: warning.follow_up,
        })),
        ...itemWarnings(items),
      ],
    },
    {
      kind: 'done',
      rows: [
        ...items
          .filter((item) => item.status === 'done' || item.status === 'skipped')
          .map((item) => ({
            key: item.subject,
            tone: 'ok' as const,
            title: item.subject,
            go: nextOf(item.subject),
            path: item.detail?.path,
          })),
        ...loose.map((followUp) => ({
          key: `${followUp.code}-${followUp.message}`,
          message: sentence(followUp.message),
          go: followUp,
        })),
      ],
    },
  ]
}
