import { CircleCheck, CircleX, TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunSteps } from '@/features/worktrees/components/run-steps'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { current, doneOf, ending, type Progress } from '@/features/worktrees/utils/progress'
import { cn } from '@/lib/utils'

export function RemoveProgress({
  progress,
  running,
  onHide,
  onCancel,
  onDone,
}: {
  progress: Progress<'clean'>
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onDone: () => void
}) {
  const names = progress.units.map((unit) => unit.subject)
  const items = progress.envelope?.items ?? []
  const removed = items.filter((item) => item.status === 'done').map((item) => item.subject)
  const failed = names.filter((name) => !removed.includes(name))
  const now = current(progress)
  const { done, total } = doneOf(progress)
  const { cancelled, failure } = ending(progress)
  const ok = !running && !failure && !cancelled
  // the run's own failure (a stale plan, a refusal); an item's shows on its line
  const refused = progress.refused ?? progress.envelope?.error?.message
  const plan = progress.envelope?.plan
  const moved = (subject: string) =>
    (plan?.reparent_children ? (plan.moves ?? []) : [])
      .filter((move) => move.from === subject)
      .map((move) => `; ${move.branch} moved onto ${move.to}`)
      .join('')

  const title = () => {
    if (running)
      return progress.cancelling ? `Cancelling the removal of ${names.join(', ')}` : `Removing ${names.join(', ')}`
    if (cancelled) return `Cancelled the removal of ${names.join(', ')}`
    if (ok) return `${names.join(', ')} ${names.length === 1 ? 'is' : 'are'} removed`
    if (removed.length === 0) return `Couldn't remove ${failed.join(', ')}`
    return `Removed ${removed.join(', ')}, couldn't remove ${failed.join(', ')}`
  }

  const description = () => {
    if (ok) return 'Each went with its local branch and its record.'
    if (!running) return `${removed.length} of ${names.length} removed.${cancelled ? ' Those not started stay.' : ''}`
    if (now) return `Step ${now.index + 1} of ${now.total} · ${phaseLabel[now.run.phase]} of ${now.subject}`
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
        {!running && refused && <p className="text-sm text-destructive">{refused}</p>}

        {!running && items.length + (progress.envelope?.warnings.length ?? 0) > 0 && (
          <ul aria-label="Outcome" className="flex flex-col gap-3 border-t pt-4">
            {items.map((item) => (
              <li key={item.subject} className="flex items-start gap-3">
                {item.status === 'done' ? (
                  <CircleCheck className="mt-0.5 size-4 text-green-600 dark:text-green-500" />
                ) : (
                  <CircleX className="mt-0.5 size-4 text-destructive" />
                )}
                <span className="flex min-w-0 flex-col">
                  <span className="text-sm font-medium">{item.subject}</span>
                  <span className={item.error ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>
                    {item.error?.message ?? `Removed, with its branch and its record${moved(item.subject)}.`}
                  </span>
                </span>
              </li>
            ))}
            {(progress.envelope?.warnings ?? []).map((warning) => (
              <li key={`${warning.code}-${warning.message}`} className="flex flex-col gap-2">
                <span className="flex items-start gap-3 text-sm">
                  <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-500" />
                  {warning.message}
                </span>
                {warning.follow_up && <CommandLine followUp={warning.follow_up} />}
              </li>
            ))}
          </ul>
        )}
      </DialogScroll>

      <DialogFooter className="items-center">
        {running ? (
          <RunningFooter
            progress={progress}
            stopping="Finishing the removal under way, then it stops."
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
