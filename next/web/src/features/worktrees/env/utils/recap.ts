import type { DomainWarning } from '@/api/types.gen'
import { type OutcomeGroup, type OutcomeRow, planWide } from '@/features/worktrees/utils/outcome'
import type { Progress } from '@/features/worktrees/utils/progress'
import { sentence } from '@/features/worktrees/utils/sentence'
import { plural } from '@/lib/plural'

// restart opens run start on a job the core says runs on the old values
export function recapOf(
  progress: Progress<'env'>,
  restart: (worktree: string, job: string) => void,
): { groups: OutcomeGroup[]; warnings: DomainWarning[]; reconciled: number } {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const written = items.filter((item) => item.status === 'done' && item.reason !== 'unchanged')

  const toCheck = items.flatMap((item) =>
    (item.warnings ?? []).map((warning): OutcomeRow => {
      const job = warning.follow_up?.answers?.find((answer) => answer.path === 'job')?.values?.[0]
      return {
        key: `${item.subject}:${warning.code}`,
        tone: 'attention',
        title: item.subject,
        message: sentence(warning.message),
        ...(job ? { act: () => restart(item.subject, job), actLabel: 'Restart' } : { followUp: warning.follow_up }),
      }
    }),
  )

  // what the plan left as it is in a worktree, the core's fix beside it
  const leftAlone = (envelope?.warnings ?? [])
    .filter((warning) => !planWide(warning))
    .map(
      (warning): OutcomeRow => ({
        key: `${warning.params?.worktree}:${warning.code}`,
        tone: 'attention',
        title: warning.params?.worktree ?? '',
        message: sentence(warning.message),
        followUp: warning.follow_up,
      }),
    )

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
              message: item.error && sentence(item.error.message),
              output: cause ? { lines: cause.split('\n') } : undefined,
            }
          }),
      },
      { kind: 'check', rows: [...toCheck, ...leftAlone] },
      {
        kind: 'done',
        rows: written.map((item) => ({
          key: item.subject,
          tone: 'ok' as const,
          title: item.subject,
          message: `${plural((item.detail?.written ?? []).length, 'file')} written`,
          path: item.detail?.path,
        })),
      },
    ],
    warnings: (envelope?.warnings ?? []).filter(planWide),
    reconciled: written.length,
  }
}
