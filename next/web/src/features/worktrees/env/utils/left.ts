import type { DomainWarning, EnvReconcile } from '@/api/types.gen'
import { keptOf, writes } from '@/features/worktrees/env/utils/plan'
import { isolationWords, leftWords } from '@/features/worktrees/env/utils/words'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'
import { sentence } from '@/features/worktrees/utils/sentence'

// what Confirm leaves as it is: each key kept, then what the core says of a worktree it leaves (folder gone, files
// unreadable, isolation pending, ports not settled or left alone) with its fix, then the verbatim ones and those in step
export function leftRowsOf(reconciles: EnvReconcile[], warnings: DomainWarning[]): OutcomeRow[] {
  const named = warnings.filter((warning) => warning.params?.worktree)
  const said = (r: EnvReconcile) => named.some((warning) => warning.params?.worktree === r.worktree)
  const quiet = reconciles.filter((r) => !writes(r) && keptOf(r).length === 0 && !said(r))
  const verbatim = quiet.filter((r) => r.isolation === 'verbatim')
  const inStep = quiet.filter((r) => r.isolation !== 'verbatim')

  return [
    ...reconciles.flatMap((r) =>
      keptOf(r).map(
        (kept): OutcomeRow => ({
          key: `${r.worktree}:${kept.file}:${kept.key}`,
          tone: 'muted',
          title: `${kept.key} · ${r.worktree}`,
          message: `${kept.file}: ${leftWords[kept.status]}`,
        }),
      ),
    ),
    ...named.map(
      (warning): OutcomeRow => ({
        key: `${warning.params?.worktree}:${warning.code}`,
        tone: 'attention',
        title: warning.params?.worktree ?? '',
        message: sentence(warning.message),
        followUp: warning.follow_up,
      }),
    ),
    ...verbatim.map(
      (r): OutcomeRow => ({
        key: `v:${r.worktree}`,
        tone: 'muted',
        title: r.worktree,
        message: isolationWords.verbatim,
      }),
    ),
    ...(inStep.length > 0
      ? [
          {
            key: 'in-step',
            tone: 'muted' as const,
            title: inStep.length === 1 ? (inStep[0]?.worktree ?? '') : `${inStep.length} other worktrees`,
            message: 'in step: nothing to rewrite',
          },
        ]
      : []),
  ]
}
