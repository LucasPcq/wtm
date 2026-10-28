import { CircleCheck, CircleMinus, CircleX, LoaderCircle, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { type Tone, toneIcon } from '@/features/worktrees/components/glyph'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { recapOf } from '@/features/worktrees/fast-forward/utils/recap'
import { outcomeWords } from '@/features/worktrees/fast-forward/utils/words'
import { doneOf, ending, type PhaseRun, type Progress } from '@/features/worktrees/utils/progress'
import { cn } from '@/lib/utils'

const stateLook: Record<PhaseRun['state'], { icon: LucideIcon; tone: Tone; words: string }> = {
  todo: { icon: CircleMinus, tone: 'muted', words: 'waiting' },
  now: { icon: LoaderCircle, tone: 'busy', words: 'fast-forwarding' },
  done: { icon: CircleCheck, tone: 'ok', words: 'fast-forwarded' },
  failed: { icon: CircleX, tone: 'failed', words: 'failed' },
}

export function FastForwardProgress({
  progress,
  running,
  onHide,
  onCancel,
  onDone,
}: {
  progress: Progress<'fast-forward'>
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onDone: () => void
}) {
  const { done, total } = doneOf(progress)
  const { cancelled, failure } = ending(progress)
  const items = progress.envelope?.items ?? []
  // the run's own failure (a stale plan, a refusal); one the core repeats from an item shows on that item's line
  const refused = progress.refused ?? (items.some((item) => item.error) ? undefined : progress.envelope?.error?.message)
  const { groups, warnings, moved } = recapOf(progress)

  const description = () => {
    if (running && progress.cancelling) return 'Cancelling: the worktree under way finishes first.'
    // taken at the click, a direct run has no step until its plan lands
    if (running) return total === 0 ? 'Planning' : `${done} of ${total}`
    if (cancelled) return 'Cancelled: what is done stays.'
    if (!failure) return `${moved} fast-forwarded.`
    return `${moved} of ${progress.units.length} fast-forwarded.`
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Fast-forward worktrees</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {running ? (
          <ul aria-label="Worktrees" className="flex flex-col gap-1">
            {progress.units.map(({ subject, phases }) => {
              const { icon: Icon, tone, words } = stateLook[phases[0]?.state ?? 'todo']
              return (
                <li key={subject} className="flex min-h-8 items-center gap-3 text-sm">
                  <Icon className={cn(toneIcon({ tone }), 'size-4')} aria-hidden="true" />
                  <span className="font-medium">{subject}</span>
                  <span className="text-muted-foreground">{words}</span>
                </li>
              )
            })}
          </ul>
        ) : (
          <>
            {/* cancelled, the description says so: the cancel's own error is no failure */}
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
          <Button onClick={onDone}>Done</Button>
        )}
      </DialogFooter>
    </>
  )
}
