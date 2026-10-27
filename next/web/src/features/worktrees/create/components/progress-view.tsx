import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunSteps } from '@/features/worktrees/components/run-steps'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { recapOf } from '@/features/worktrees/create/utils/recap'
import { outcomeWords } from '@/features/worktrees/create/utils/words'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { current, doneOf, ending, type Progress } from '@/features/worktrees/utils/progress'

export function ProgressView({
  progress,
  running,
  onHide,
  onCancel,
  onAnother,
  onDone,
}: {
  progress: Progress<'create'>
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onAnother: () => void
  onDone: (branch: string | undefined) => void
}) {
  const branches = progress.units
    .filter((unit) => unit.phases.some((run) => run.phase !== 'fast_forward'))
    .map((unit) => unit.subject)
  const names = branches.join(', ')
  const now = current(progress)
  const { done, total } = doneOf(progress)
  const { cancelled, failure } = ending(progress)
  const ok = !running && !failure && !cancelled

  const description = () => {
    if (ok) return `${names} ${branches.length === 1 ? 'is' : 'are'} ready.`
    if (running && progress.cancelling) return `Cancelling ${names}`
    if (!running) {
      const succeeded = progress.units.flatMap((unit) => unit.phases).filter((run) => run.state === 'done').length
      return `${succeeded} of ${total} steps done.${cancelled ? ' What is done stays: clean it to remove it.' : ''}`
    }
    if (now) return `Step ${now.index + 1} of ${now.total} · ${phaseLabel[now.run.phase]}`
    return done > 0 ? 'Finishing…' : 'Starting…'
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Create worktrees</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {!running && (
          <RunFailure message={(progress.envelope?.items ?? []).some((item) => item.error) ? undefined : failure} />
        )}
        {!running && (
          <RunOutcome groups={recapOf(progress)} labels={outcomeWords} warnings={progress.envelope?.warnings} />
        )}
        <RunSteps progress={progress} running={running} />
      </DialogScroll>

      <DialogFooter className="items-center">
        {running ? (
          <RunningFooter
            progress={progress}
            stopping="Finishing the step under way, then it stops. What is done stays."
            onHide={onHide}
            onCancel={onCancel}
          />
        ) : (
          <>
            <Button variant="outline" className="mr-auto" onClick={onAnother}>
              Create another
            </Button>
            <Button onClick={() => onDone(ok ? branches[0] : undefined)}>Done</Button>
          </>
        )}
      </DialogFooter>
    </>
  )
}
