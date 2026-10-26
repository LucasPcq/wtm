import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunSteps } from '@/features/worktrees/components/run-steps'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { Made } from '@/features/worktrees/create/components/made'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { current, doneOf, ending, type Progress } from '@/features/worktrees/utils/progress'
import { cn } from '@/lib/utils'

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

  const title = () => {
    if (running) return progress.cancelling ? `Cancelling ${names}` : `Creating ${names}`
    if (cancelled) return `Cancelled ${names}`
    if (failure) return `Couldn't create ${names}`
    return `${names} ${branches.length === 1 ? 'is' : 'are'} ready`
  }

  const description = () => {
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
        <DialogTitle>{title()}</DialogTitle>
        <DialogDescription className={cn(ok && 'sr-only')}>{description()}</DialogDescription>
      </DialogHeader>
      {running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}

      <DialogScroll>
        <RunSteps progress={progress} ok={ok} />
        {!running && failure && <p className="text-sm text-destructive">{failure}</p>}
        {!running && progress.envelope && <Made envelope={progress.envelope} />}
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
