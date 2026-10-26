import { Database } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunSteps } from '@/features/worktrees/components/run-steps'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { listed } from '@/features/worktrees/remove/utils/listed'
import { recapOf } from '@/features/worktrees/remove/utils/recap'
import { outcomeWords } from '@/features/worktrees/remove/utils/words'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { current, doneOf, ending, type Progress } from '@/features/worktrees/utils/progress'

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
  // the run's own failure (a stale plan, a refusal); one the core repeats from an item shows on that item's line
  const refused = progress.refused ?? (items.some((item) => item.error) ? undefined : progress.envelope?.error?.message)
  const { groups, kept } = recapOf(progress)
  // a worktree on no branch, or a branch with no worktree, is no "worktree with its branch": then only the count holds
  const plain = (progress.envelope?.plan?.worktrees ?? [])
    .filter((removal) => removed.includes(removal.worktree))
    .every((removal) => removal.branch && removal.path)

  const title = () => {
    if (running) return progress.cancelling ? `Cancelling the removal of ${listed(names)}` : `Removing ${listed(names)}`
    if (cancelled) return `Cancelled the removal of ${listed(names)}`
    if (ok) return `Removed ${listed(names)}`
    if (removed.length === 0) return `Couldn't remove ${listed(failed)}`
    return `Removed ${listed(removed)}, couldn't remove ${listed(failed)}`
  }

  const description = () => {
    if (ok && !plain) return `${removed.length} removed.`
    if (ok)
      return names.length === 1
        ? 'Its local branch went with it; the remote branch stays.'
        : 'Their local branches went with them; the remote branches stay.'
    if (!running) return `${removed.length} of ${names.length} removed.${cancelled ? ' Those not started stay.' : ''}`
    if (now) return `Step ${now.index + 1} of ${now.total} · ${phaseLabel[now.run.phase]} of ${now.subject}`
    return done > 0 ? 'Finishing…' : 'Starting…'
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title()}</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      {running && (
        <DialogBand>
          <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />
        </DialogBand>
      )}

      <DialogScroll>
        {!running && <RunFailure message={refused} />}
        {!running && <RunOutcome groups={groups} labels={outcomeWords} />}
        <RunSteps progress={progress} running={running}>
          {kept.map((line) => (
            <p key={line} className="flex items-start gap-2 text-xs text-muted-foreground">
              <Database className="mt-px size-3.5 shrink-0" aria-hidden="true" />
              {line}
            </p>
          ))}
        </RunSteps>
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
