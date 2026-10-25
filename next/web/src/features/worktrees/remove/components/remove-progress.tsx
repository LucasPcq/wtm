import { ChevronRight, CircleCheck, CircleX, Square, TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { Timeline } from '@/features/worktrees/components/timeline'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { current, doneOf, ending, type Progress, took } from '@/features/worktrees/utils/progress'

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
  const plan = progress.envelope?.plan
  const moved = (subject: string) =>
    (plan?.reparent_children ? (plan.moves ?? []) : [])
      .filter((move) => move.from === subject)
      .map((move) => `; ${move.branch} moved onto ${move.to}`)
      .join('')

  const title = () => {
    if (running)
      return progress.cancelling ? `Cancelling the removal of ${names.join(', ')}` : `Removing ${names.join(', ')}`
    if (ok) return `${names.join(', ')} ${names.length === 1 ? 'is' : 'are'} removed`
    if (removed.length === 0) return `Couldn't remove ${failed.join(', ')}`
    return `Removed ${removed.join(', ')}, couldn't remove ${failed.join(', ')}`
  }

  const step = () => {
    if (now) return `Step ${now.index + 1} of ${now.total} · ${phaseLabel[now.run.phase]} of ${now.subject}`
    return done > 0 ? 'Finishing…' : 'Starting…'
  }

  const footer = () => {
    if (!running) return <Button onClick={onDone}>Done</Button>

    return (
      <>
        <p className="mr-auto text-sm text-muted-foreground">
          {progress.cancelling ? 'Finishing the removal under way, then it stops.' : 'Closing this keeps it running.'}
        </p>
        <Button variant="ghost" onClick={onHide}>
          Hide
        </Button>
        {progress.cancelling ? (
          <Button variant="destructive" disabled={progress.cancelling === 'forced'} onClick={() => onCancel(true)}>
            Stop now
          </Button>
        ) : (
          <Button variant="outline" disabled={!progress.operation} onClick={() => onCancel(false)}>
            <Square />
            Cancel
          </Button>
        )}
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title()}</DialogTitle>
        <DialogDescription className={ok ? 'sr-only' : undefined}>
          {running ? step() : 'Each went with its local branch and its record.'}
        </DialogDescription>
      </DialogHeader>
      {running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}

      <ScrollArea className="min-h-0 flex-1 [&_[data-slot=scroll-area-viewport]>div]:!block">
        <div className="flex flex-col gap-4 pr-3">
          {ok ? (
            <Collapsible>
              <CollapsibleTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className="-ml-2 text-muted-foreground [&[data-state=open]>svg]:rotate-90"
                >
                  <ChevronRight className="transition-transform motion-reduce:transition-none" />
                  {[`${total} steps`, took(progress)].filter(Boolean).join(' · ')}
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className="pt-3">
                <Timeline units={progress.units} />
              </CollapsibleContent>
            </Collapsible>
          ) : (
            <Timeline units={progress.units} />
          )}

          {!running && (
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
          {!running && progress.refused && <p className="text-sm text-destructive">{progress.refused}</p>}
        </div>
      </ScrollArea>

      <DialogFooter className="items-center">{footer()}</DialogFooter>
    </>
  )
}
