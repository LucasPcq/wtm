import { phaseDuring } from '@/features/worktrees/create/utils/words'
import { type OutcomeGroup, outputOf } from '@/features/worktrees/utils/outcome'
import type { Progress, Unit } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

const stoppedIn = (unit: Unit | undefined) =>
  unit?.phases.find((run) => run.state === 'now' || run.state === 'failed') ??
  unit?.phases.findLast((run) => run.startedAt !== undefined)

// what a create that ended did, in the groups the dialog shows: each worktree ready with the step into it, each one
// that failed or was cancelled with its output and that step, then the warnings
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
    const phase = stoppedIn(unit)?.phase
    const cancelled = item.status === 'cancelled'
    return {
      key: item.subject,
      tone: cancelled ? ('stopped' as const) : ('failed' as const),
      title: item.subject,
      message: cancelled
        ? phase && `Cancelled during ${phaseDuring[phase]}`
        : item.error && sentence(item.error.message),
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
        ...loose.map((followUp) => ({
          key: `${followUp.code}-${followUp.message}`,
          tone: 'attention' as const,
          message: sentence(followUp.message),
          followUp,
        })),
      ],
    },
    {
      kind: 'done',
      rows: items
        .filter((item) => item.status === 'done' || item.status === 'skipped')
        .map((item) => ({
          key: item.subject,
          tone: 'ok' as const,
          title: item.subject,
          go: nextOf(item.subject),
          path: item.detail?.path,
        })),
    },
  ]
}
