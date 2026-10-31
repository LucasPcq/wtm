import type { DomainAnswered, DomainPhase } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { RunningUnits, type UnitState } from '@/features/worktrees/components/running-units'
import { recapOf } from '@/features/worktrees/sync/utils/recap'
import { outcomeWords } from '@/features/worktrees/sync/utils/words'
import { doneOf, ending, type Progress, type Unit } from '@/features/worktrees/utils/progress'

const stateOf = (unit: Unit): UnitState => {
  if (unit.phases.some((run) => run.state === 'failed')) return 'failed'
  if (unit.phases.length > 0 && unit.phases.every((run) => run.state === 'done')) return 'done'
  if (unit.phases.some((run) => run.state === 'now' || run.state === 'done')) return 'now'
  return 'todo'
}

// running, the same flat list as Confirm in run order, each row saying where it goes
export function SyncProgress({
  progress,
  running,
  onHide,
  onCancel,
  onDone,
  onFollow,
}: {
  progress: Progress<'sync'>
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onDone: () => void
  onFollow: (answers: DomainAnswered[]) => void
}) {
  const { done, total } = doneOf(progress)
  const { cancelled } = ending(progress)
  const items = progress.envelope?.items ?? []
  const refused = progress.refused ?? (items.some((item) => item.error) ? undefined : progress.envelope?.error?.message)
  const { groups, warnings, summary } = recapOf(progress, onFollow)
  const rebaseUnits = progress.units.filter((unit) => unit.phases.some((run) => run.phase === 'rebase'))

  // each row from its own phases: a rebase onto its parent, its own origin's commits taken, a push alone
  const words = (unit: Unit, state: UnitState) => {
    const has = (phase: DomainPhase) => unit.phases.some((run) => run.phase === phase)
    const now = unit.phases.find((run) => run.state === 'now')?.phase
    if (state === 'failed') return 'failed'
    if (state === 'todo') return has('rebase') && unit.onto ? `waits for ${unit.onto}` : 'waiting'
    if (state === 'done') {
      if (has('rebase')) return `onto ${unit.onto}`
      return has('fast_forward') ? 'fast-forwarded to origin' : 'pushed to origin'
    }
    if (now === 'push') return 'pushing to origin'
    if (now === 'fast_forward') return has('rebase') ? "taking origin's commits" : 'fast-forwarding to origin'
    return `rebasing onto ${unit.onto ?? 'its parent'}`
  }

  const under = () => {
    const at = progress.units.find((unit) => {
      const state = stateOf(unit)
      return state === 'now' || state === 'todo'
    })
    if (!at) return `${rebaseUnits.length} of ${rebaseUnits.length}`
    const pushing = at.phases.find((run) => run.state === 'now')?.phase === 'push'
    const k = rebaseUnits.indexOf(at)
    if (k < 0) return `${pushing ? 'Pushing' : 'Fast-forwarding'} ${at.subject}`
    return `${k + 1} of ${rebaseUnits.length} · ${pushing ? 'pushing' : 'rebasing'} ${at.subject}${!pushing && at.onto ? ` onto ${at.onto}` : ''}`
  }

  const description = () => {
    if (running && progress.cancelling) return 'Cancelling: the worktree under way finishes first.'
    if (running) return total === 0 ? 'Planning' : under()
    return summary
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Sync worktrees</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {running ? (
          <RunningUnits
            rows={progress.units.map((unit) => {
              const state = stateOf(unit)
              return { subject: unit.subject, state, words: words(unit, state) }
            })}
          />
        ) : (
          <>
            {!cancelled && <RunFailure message={refused} />}
            <RunOutcome groups={groups} labels={outcomeWords} warnings={warnings} />
          </>
        )}
      </DialogScroll>

      <DialogFooter className="items-center">
        {running ? (
          <RunningFooter
            progress={progress}
            stopping="Finishing the rebase under way, then it stops."
            onHide={onHide}
            onCancel={onCancel}
          />
        ) : (
          <Button onClick={onDone}>Done</Button>
        )}
      </DialogFooter>
    </>
  )
}
