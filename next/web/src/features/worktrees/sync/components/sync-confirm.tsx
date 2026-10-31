import { ArrowDownToLine, CirclePause, Cloud, CloudOff, ListTree } from 'lucide-react'
import type { DomainFollowUp, DomainWarning, SyncPlan, SyncRebase } from '@/api/types.gen'
import { Details } from '@/features/worktrees/components/details'
import { Facts } from '@/features/worktrees/components/facts'
import { LeftRow } from '@/features/worktrees/components/left-row'
import { LeftSection } from '@/features/worktrees/components/left-section'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { PlanRow } from '@/features/worktrees/sync/components/plan-row'
import { PlanSentence } from '@/features/worktrees/sync/components/plan-sentence'
import { leftRowsOf } from '@/features/worktrees/sync/utils/left'
import { leftOutWords, markedOf, moves, rootOf, unsafe } from '@/features/worktrees/sync/utils/plan'
import { phaseWords, sentenceWords } from '@/features/worktrees/sync/utils/words'
import { commits } from '@/features/worktrees/utils/counts'
import { listed } from '@/features/worktrees/utils/listed'
import { type OutcomeRow, planWide } from '@/features/worktrees/utils/outcome'
import { phaseLabel } from '@/features/worktrees/utils/phases'

// Confirm holds no control: the plan in reading form, every choice made on step 1
export function SyncConfirm({
  plan,
  warnings,
  reviewWarnings,
  left,
  review,
  behind,
  changesOf,
  onReparent,
}: {
  plan: SyncPlan
  warnings: DomainWarning[]
  reviewWarnings: DomainWarning[]
  left: OutcomeRow[]
  review: SyncRebase[]
  behind: string[]
  changesOf: (worktree: string) => number | undefined
  onReparent: (followUp: DomainFollowUp) => void
}) {
  const forwarded = (plan.onto ?? []).filter(moves)
  const running = (plan.worktrees ?? []).filter(moves)
  const asItStands = behind.filter((branch) => !forwarded.some((onto) => onto.branch === branch))
  // the review's descendants: one checked anyway is named, yet follows all the same
  const marked = markedOf(plan, review)
  const under = marked.filter((rebase) => rebase.descendant).map((rebase) => rebase.worktree)
  const followers = running.filter((rebase) => under.includes(rebase.worktree))
  const kept = (plan.worktrees ?? []).filter((rebase) => rebase.descendant && !moves(rebase) && unsafe(rebase))
  // as Left as they are says them: those that need a sync after; one with its own reason says it in the list
  const leftOut = (plan.left_out ?? []).filter((worktree) =>
    leftOutWords(
      plan,
      review.find((rebase) => rebase.worktree === worktree),
      worktree,
    ),
  )

  // the count rule: only a rebase takes a number; a worktree that only takes its own origin's commits, or only pushes,
  // has its glyph
  const rebasing = running.filter((rebase) => (rebase.phases ?? []).includes('rebase'))
  const leadOf = (rebase: SyncRebase) => {
    const at = rebasing.indexOf(rebase)
    if (at >= 0) return at + 1
    return (rebase.phases ?? []).includes('push') && !(rebase.phases ?? []).includes('fast_forward')
      ? 'push'
      : 'forward'
  }

  const message = (rebase: SyncRebase) => {
    const onto = asItStands.includes(rebase.parent) ? `${rebase.parent} as it stands` : rebase.parent
    return [
      phaseWords(rebase, onto),
      under.includes(rebase.worktree) && `follows ${rootOf(marked, rebase.worktree)}`,
      unsafe(rebase) && 'its changes stashed',
    ]
      .filter(Boolean)
      .join(' · ')
  }

  const descendants = () => {
    if (!plan.descendants)
      return leftOut.length > 0
        ? `${listed(leftOut)} ${leftOut.length === 1 ? 'stays where it is, and needs' : 'stay where they are; each needs'} a sync after.`
        : undefined
    const byRoot = new Map<string, typeof followers>()
    for (const rebase of followers) {
      const root = rootOf(marked, rebase.worktree)
      byRoot.set(root, [...(byRoot.get(root) ?? []), rebase])
    }
    const parts = [
      ...[...byRoot].map(
        ([root, rows]) => `${listed(rows.map((r) => r.worktree))} ${rows.length === 1 ? 'follows' : 'follow'} ${root}`,
      ),
      ...kept.map((rebase) => `${rebase.worktree} is kept (uncommitted changes)`),
    ]
    return parts.length > 0 ? `${parts.join('; ')}.` : undefined
  }
  const descendantsSentence = descendants()

  return (
    <div className="flex flex-col gap-6">
      <PlanWarnings warnings={warnings.filter(planWide)} />

      <section aria-label="In this order" className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-muted-foreground">In this order</h3>
        <ol className="flex flex-col gap-1">
          {forwarded.map((onto) => (
            <PlanRow
              key={onto.branch}
              lead="forward"
              title={onto.branch}
              message={[
                `fast-forwarded to origin first · ${commits(onto.origin?.behind ?? 0)}`,
                (onto.blockers ?? []).length > 0 && 'its changes stashed',
              ]
                .filter(Boolean)
                .join(' · ')}
            />
          ))}
          {running.map((rebase) => (
            <PlanRow key={rebase.worktree} lead={leadOf(rebase)} title={rebase.worktree} message={message(rebase)} />
          ))}
        </ol>
      </section>

      <LeftSection>
        {[
          ...leftRowsOf({ plan, review, warnings: [...warnings, ...reviewWarnings], changesOf, onReparent }),
          ...left,
        ].map((row) => (
          <LeftRow key={row.key} row={row} />
        ))}
      </LeftSection>

      <section aria-label="How it runs" className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-muted-foreground">How it runs</h3>
        <ul className="flex flex-col gap-2">
          {descendantsSentence && <PlanSentence icon={ListTree}>{descendantsSentence}</PlanSentence>}
          {asItStands.length > 0 && (
            <PlanSentence icon={ArrowDownToLine}>{sentenceWords.asItStands(asItStands)}</PlanSentence>
          )}
          {rebasing.length === 0 && plan.push && <PlanSentence icon={Cloud}>{sentenceWords.pushOnly}</PlanSentence>}
          {rebasing.length === 0 && !plan.push && forwarded.length > 0 && (
            <PlanSentence icon={ArrowDownToLine}>
              {sentenceWords.forwardOnly(forwarded.map((onto) => onto.branch))}
            </PlanSentence>
          )}
          {rebasing.length > 0 && (
            <PlanSentence icon={plan.push ? Cloud : CloudOff}>
              {plan.push ? sentenceWords.pushed : sentenceWords.notPushed}
            </PlanSentence>
          )}
          {rebasing.length > 0 && (
            <PlanSentence icon={CirclePause}>
              {plan.keep_conflict ? sentenceWords.kept : sentenceWords.stopped}
            </PlanSentence>
          )}
        </ul>
      </section>

      <Details label="Details" summary="the steps of each worktree">
        {[...forwarded, ...running].map((row) => {
          const name = 'branch' in row ? row.branch : row.worktree
          return (
            <Facts
              key={name}
              heading={name}
              facts={[
                ...(row.path ? [{ label: 'Folder', values: [row.path], mono: true }] : []),
                {
                  label: 'Steps',
                  values: (row.phases ?? []).map((phase) =>
                    phase === 'fast_forward' ? "Take origin's commits" : phaseLabel[phase],
                  ),
                },
              ]}
            />
          )
        })}
      </Details>
    </div>
  )
}
