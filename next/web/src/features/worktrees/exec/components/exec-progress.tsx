import { useQuery } from '@tanstack/react-query'
import { RotateCcw } from 'lucide-react'
import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { TargetRow } from '@/features/worktrees/exec/components/target-row'
import { recapOf } from '@/features/worktrees/exec/utils/recap'
import { passedWords, retryCode } from '@/features/worktrees/exec/utils/words'
import type { Ran } from '@/features/worktrees/hooks/use-exec-dialog'
import { subjectsOf } from '@/features/worktrees/utils/actions'
import { doneOf, ending, type Progress } from '@/features/worktrees/utils/progress'

export function ExecProgress({
  progress,
  running,
  ran,
  onHide,
  onCancel,
  onDone,
  onAgain,
}: {
  progress: Progress<'exec'>
  running: boolean
  ran: Ran
  onHide: () => void
  onCancel: (force: boolean) => void
  onDone: () => void
  onAgain: (retry: DomainFollowUp) => void
}) {
  // the elapsed time of each command under way follows the clock, not the events
  const now =
    useQuery({ queryKey: ['now', 'second'], queryFn: () => Date.now(), refetchInterval: 1000, enabled: running })
      .data ?? Date.now()

  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const units = progress.units
  const { done, total } = doneOf(progress)
  const { cancelled } = ending(progress)
  const failed = items.filter((item) => item.status === 'failed').length
  const retry = envelope?.follow_ups?.find((followUp) => followUp.code === retryCode)
  const again = retry ? ` Run again takes the ${subjectsOf(retry).length} that didn't pass.` : ''
  // the run's own failure (a stale plan, a refusal); one the core repeats from an item shows on that item's line
  const refused =
    progress.refused ?? (cancelled || items.some((item) => item.error) ? undefined : envelope?.error?.message)
  const { groups, labels, warnings } = recapOf(envelope)

  const stopping = () => {
    const queued = units.filter((unit) => unit.phases.every((run) => run.state === 'todo')).length
    if (queued === 0) return 'Stopping the commands under way. What is done stays.'
    const wont = queued === 1 ? "the queued one won't start" : `the ${queued} queued won't start`
    return `Stopping the commands under way; ${wont}. What is done stays.`
  }

  const description = () => {
    if (running && progress.cancelling) return `Cancelling ${ran.command} · ${done} of ${total} done`
    if (running) return `${ran.command} · ${done} of ${total} done · ${ran.jobs} at once`
    if (items.length === 0) return 'Nothing ran.'
    if (cancelled) return `Cancelled: what is done stays.${again}`
    if (failed > 0) return `${ran.command} failed in ${failed} of ${items.length}.${again}`
    return passedWords(ran.command, items)
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Run a command</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {running ? (
          <ul aria-label="Worktrees" className="flex flex-col gap-1">
            {units.map((unit) => (
              <TargetRow key={unit.subject} unit={unit} now={now} cancelling={progress.cancelling !== undefined} />
            ))}
          </ul>
        ) : (
          <>
            <RunFailure message={refused} />
            <RunOutcome groups={groups} labels={labels} warnings={warnings} />
          </>
        )}
      </DialogScroll>

      <DialogFooter className="items-center">
        {running ? (
          <RunningFooter progress={progress} stopping={stopping()} onHide={onHide} onCancel={onCancel} />
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
