import type { DomainWarning, EnvPlan } from '@/api/types.gen'
import { inStepOf, lineOf, recapLines } from '@/features/worktrees/env/utils/plan'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'
import { sentence } from '@/features/worktrees/utils/sentence'

// what Confirm leaves as it is: each key the core's recap leaves, then what the core says of a worktree it leaves
// (folder gone, files unreadable, isolation pending, ports not settled or left alone) with its fix, then those in step
export function leftRowsOf(plan: EnvPlan, warnings: DomainWarning[]): OutcomeRow[] {
  const named = warnings.filter((warning) => warning.params?.worktree && warning.code !== 'env.restart')
  const said = (worktree: string) => named.some((warning) => warning.params?.worktree === worktree)
  const inStep = inStepOf(plan).filter((worktree) => !said(worktree))

  return [
    ...[...recapLines(plan)].flatMap(([worktree, values]) =>
      values
        .filter((value) => value.note === 'left')
        .map((value): OutcomeRow => {
          const { file, change } = lineOf(value)
          return {
            key: `${worktree}:${file}:${value.value}`,
            tone: 'muted',
            title: worktree,
            message: `${file}: ${change}`,
          }
        }),
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
    ...(inStep.length > 0
      ? [
          {
            key: 'in-step',
            tone: 'muted' as const,
            title: inStep.length === 1 ? (inStep[0] ?? '') : `${inStep.length} other worktrees`,
            message: 'in step: nothing to rewrite',
          },
        ]
      : []),
  ]
}
