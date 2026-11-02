import type { Applied } from '@/api/queries/commands'
import type { DomainWarning, FastforwardAdvance } from '@/api/types.gen'
import { unsafe } from '@/features/worktrees/fast-forward/utils/plan'
import { failureWords, keptWords, standingTone, standingWords } from '@/features/worktrees/fast-forward/utils/words'
import type { Told } from '@/features/worktrees/hooks/use-direct-run'
import { commits } from '@/features/worktrees/utils/counts'
import { type OutcomeGroup, type OutcomeRow, planWide } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'

// a worktree left as it is, kept for what makes it unsafe or for where it stands, as the screen words it
export const leftRow = (advance: FastforwardAdvance, warning?: DomainWarning): OutcomeRow => ({
  key: advance.worktree,
  tone: unsafe(advance) ? 'attention' : standingTone[advance.standing],
  title: advance.worktree,
  message: unsafe(advance) ? keptWords(advance.blockers ?? []) : standingWords[advance.standing](advance),
  followUp: warning?.follow_up,
})

export function recapOf(progress: Progress<'fast-forward'>): {
  groups: OutcomeGroup[]
  warnings: DomainWarning[]
  moved: number
} {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const warnings = envelope?.warnings ?? []
  const advanceOf = (worktree: string | undefined) =>
    envelope?.plan?.worktrees?.find((advance) => advance.worktree === worktree)
  const moved = items.filter((item) => item.status === 'done' && item.reason !== 'unchanged')

  const toCheck = warnings
    .filter((warning) => !planWide(warning))
    .map((warning) => {
      const advance = advanceOf(warning.params?.worktree)
      if (advance) return leftRow(advance, warning)
      return { key: warning.code, tone: 'attention' as const, message: sentence(warning.message) }
    })

  return {
    groups: [
      {
        kind: 'failed',
        rows: items
          .filter((item) => item.status === 'failed' || item.status === 'cancelled' || item.reason === 'interrupted')
          .map((item) => {
            if (item.status !== 'failed')
              return {
                key: item.subject,
                tone: 'muted' as const,
                title: item.subject,
                message: 'not reached: cancelled first',
              }
            const cause = item.error?.params?.cause
            return {
              key: item.subject,
              tone: 'failed' as const,
              title: item.subject,
              message: item.error && (failureWords[item.error.code] ?? sentence(item.error.message)),
              output: cause ? { lines: cause.split('\n') } : undefined,
            }
          }),
      },
      { kind: 'check', rows: toCheck },
      {
        kind: 'done',
        rows: moved.map((item) => ({
          key: item.subject,
          tone: 'ok' as const,
          title: item.subject,
          message: commits(advanceOf(item.subject)?.origin?.behind ?? 0),
          path: item.detail?.path,
        })),
      },
    ],
    warnings: warnings.filter(planWide),
    moved: moved.length,
  }
}

// the toasts of one worktree fast-forwarded at once
export function toldOf(worktree: string, envelope: Applied<'fast-forward'> | undefined): Told {
  const item = envelope?.items.find((one) => one.subject === worktree)
  const taken = envelope?.plan?.worktrees?.find((advance) => advance.worktree === worktree)?.origin?.behind ?? 0
  if (item?.status === 'done')
    return {
      tone: 'success',
      title: `${worktree} is on origin's tip`,
      description: `${commits(taken)} taken from origin`,
    }
  if (envelope?.error?.kind === 'cancelled' || item?.reason === 'interrupted')
    return { tone: 'info', title: `Fast-forward of ${worktree} cancelled`, description: 'Nothing moved' }

  const error = item?.error ?? envelope?.error
  return {
    tone: 'error',
    title: `Couldn't fast-forward ${worktree}`,
    description: error ? (failureWords[error.code] ?? sentence(error.message)) : 'It stopped before it ran',
  }
}
