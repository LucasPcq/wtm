import { itemWarnings, type OutcomeGroup, outputOf, stoppedMessage } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

// the run's warnings are RunOutcome's; a next step no item claims goes with what went
export function recapOf(progress: Progress<'create'>): OutcomeGroup[] {
  const envelope = progress.envelope
  if (!envelope) return []

  const { items, follow_ups: followUps } = envelope
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
      rows: itemWarnings(items),
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
