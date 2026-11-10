import { RotateCcw } from 'lucide-react'
import type { DomainFollowUp } from '@/api/types.gen'
import { DialogBand } from '@/components/dialog-band'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { RunningUnits, type UnitState } from '@/features/worktrees/components/running-units'
import { recapOf } from '@/features/worktrees/env/utils/recap'
import { outcomeWords } from '@/features/worktrees/env/utils/words'
import { doneOf, ending, type Progress } from '@/features/worktrees/utils/progress'
import { plural } from '@/lib/plural'

const stateWords: Record<UnitState, string> = {
  todo: 'waiting',
  now: 'writing',
  done: 'reconciled',
  failed: 'failed',
}

export function EnvProgress({
  progress,
  running,
  onHide,
  onCancel,
  onDone,
  onRestart,
  onAgain,
}: {
  progress: Progress<'env'>
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onDone: () => void
  onRestart: (worktree: string, job: string) => void
  onAgain: (retry: DomainFollowUp) => void
}) {
  const { done, total } = doneOf(progress)
  const { cancelled, failure } = ending(progress)
  const items = progress.envelope?.items ?? []
  const refused = progress.refused ?? (items.some((item) => item.error) ? undefined : progress.envelope?.error?.message)
  const { groups, warnings, reconciled } = recapOf(progress, onRestart)
  const retry = progress.envelope?.follow_ups?.find((followUp) => followUp.code === 'env.retry')

  const description = () => {
    if (running && progress.cancelling) return 'Cancelling: the worktree under way finishes first.'
    if (running) return `${done} of ${total}`
    if (cancelled) return 'Cancelled: what is written stays.'
    if (!failure) return `${plural(reconciled, 'worktree')} reconciled.`
    return `${reconciled} of ${plural(progress.units.length, 'worktree')} reconciled.`
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Reconcile .env files</DialogTitle>
        <DialogDescription role="status">{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {running ? (
          <RunningUnits
            rows={progress.units.map(({ subject, phases }) => {
              const state = phases.find((phase) => phase.state !== 'done')?.state ?? phases.at(-1)?.state ?? 'todo'
              return { subject, state, words: stateWords[state] }
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
            stopping="Finishing the worktree under way, then it stops."
            onHide={onHide}
            onCancel={onCancel}
          />
        ) : (
          <>
            {retry && (
              <Button variant="outline" onClick={() => onAgain(retry)}>
                <RotateCcw />
                Run again
              </Button>
            )}
            <Button onClick={onDone}>Done</Button>
          </>
        )}
      </DialogFooter>
    </>
  )
}
