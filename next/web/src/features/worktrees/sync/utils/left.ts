import type { DomainFollowUp, DomainWarning, SyncPlan, SyncRebase } from '@/api/types.gen'
import { moves, unsafe } from '@/features/worktrees/sync/utils/plan'
import { standingTone, standingWords } from '@/features/worktrees/sync/utils/words'
import { changes } from '@/features/worktrees/utils/counts'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'

const reparentLabel = (followUp: DomainFollowUp) => {
  const to = followUp.answers?.find((answer) => answer.path === 'to')?.values[0]
  return to ? `Reparent onto ${to}` : 'Reparent'
}

// the worktrees the plan leaves as they are, each with why and its fix: a reparent runs in the dialog, anything else
// is copied
export function leftRowsOf({
  plan,
  review,
  warnings,
  changesOf,
  onReparent,
}: {
  plan: SyncPlan
  review: SyncRebase[]
  warnings: DomainWarning[]
  changesOf: (worktree: string) => number | undefined
  onReparent: (followUp: DomainFollowUp) => void
}): OutcomeRow[] {
  const inPlan = (worktree: string) => (plan.worktrees ?? []).some((rebase) => rebase.worktree === worktree)
  // a worktree step 1 showed as one it cannot rebase (mid-rebase, folder gone, diverged…) is left all the same when the
  // command's own pick never named it
  const kept = [
    ...(plan.worktrees ?? []).filter((rebase) => !moves(rebase)),
    ...review.filter(
      (rebase) =>
        !inPlan(rebase.worktree) &&
        !(plan.left_out ?? []).includes(rebase.worktree) &&
        !['rebases', 'on_parent', 'in_cycle'].includes(rebase.standing),
    ),
  ]
  return [
    ...kept.map((rebase): OutcomeRow => {
      const followUp = warnings.find((warning) => warning.params?.worktree === rebase.worktree)?.follow_up
      const reparent = followUp?.command === 'reparent' ? followUp : undefined
      return {
        key: rebase.worktree,
        tone: unsafe(rebase) ? 'attention' : standingTone[rebase.standing],
        title: rebase.worktree,
        message: unsafe(rebase)
          ? `${changes(changesOf(rebase.worktree))}: kept · needs a sync after`
          : standingWords[rebase.standing](rebase),
        // a dirty one kept is checked on step 1, never forced from here
        followUp: unsafe(rebase) || reparent ? undefined : followUp,
        act: reparent && (() => onReparent(reparent)),
        actLabel: reparent && reparentLabel(reparent),
      }
    }),
    ...(plan.left_out ?? []).map((worktree): OutcomeRow => {
      const rebase = review.find((row) => row.worktree === worktree)
      const parent = rebase?.parent ?? ''
      // its parent moves in this run: it stays on the old tip; else it stays with its parent, for the reason the review
      // gives
      const moving = (plan.worktrees ?? []).some((row) => row.worktree === parent && moves(row))
      if (rebase && !moving && rebase.standing !== 'rebases')
        return {
          key: worktree,
          tone: standingTone[rebase.standing],
          title: worktree,
          message: standingWords[rebase.standing](rebase),
        }
      return {
        key: worktree,
        tone: 'attention',
        title: worktree,
        message: `stays on ${parent}'s old tip · needs a sync after`,
      }
    }),
  ]
}
