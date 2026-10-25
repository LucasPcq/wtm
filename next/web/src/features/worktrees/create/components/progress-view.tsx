import { ChevronRight, Square } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Made } from '@/features/worktrees/create/components/made'
import { Timeline } from '@/features/worktrees/create/components/timeline'
import { current, doneOf, ending, type Progress, took } from '@/features/worktrees/create/utils/progress'
import { phaseLabel } from '@/features/worktrees/create/utils/words'
import { cn } from '@/lib/utils'

export function ProgressView({
  progress,
  running,
  onHide,
  onCancel,
  onAnother,
  onDone,
}: {
  progress: Progress
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

  const stop = () => {
    if (!progress.cancelling)
      return (
        <Button variant="outline" disabled={!progress.operation} onClick={() => onCancel(false)}>
          <Square />
          Cancel
        </Button>
      )

    return (
      <Button variant="destructive" disabled={progress.cancelling === 'forced'} onClick={() => onCancel(true)}>
        Stop now
      </Button>
    )
  }

  const footer = () => {
    if (!running)
      return (
        <>
          <Button variant="outline" className="mr-auto" onClick={onAnother}>
            Create another
          </Button>
          <Button onClick={() => onDone(ok ? branches[0] : undefined)}>Done</Button>
        </>
      )

    return (
      <>
        <p className="mr-auto text-sm text-muted-foreground">
          {progress.cancelling
            ? 'Finishing the step under way, then it stops. What is done stays.'
            : 'Closing this keeps it running.'}
        </p>
        <Button variant="ghost" onClick={onHide}>
          Hide
        </Button>
        {stop()}
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title()}</DialogTitle>
        <DialogDescription className={cn(ok && 'sr-only')}>{description()}</DialogDescription>
      </DialogHeader>
      {running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}

      {/* Radix lays the viewport's content out as a table, which never narrows: a long path would push the
          commands out of sight instead of truncating */}
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
                <Timeline progress={progress} />
              </CollapsibleContent>
            </Collapsible>
          ) : (
            <Timeline progress={progress} />
          )}
          {!running && failure && <p className="text-sm text-destructive">{failure}</p>}
          {!running && progress.envelope && <Made envelope={progress.envelope} />}
        </div>
      </ScrollArea>

      <DialogFooter className="items-center">{footer()}</DialogFooter>
    </>
  )
}
