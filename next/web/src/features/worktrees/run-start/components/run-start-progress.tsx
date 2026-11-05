import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { JobSection } from '@/features/worktrees/components/job-section'
import { PhaseRow } from '@/features/worktrees/components/phase-row'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import type { RunStartRan } from '@/features/worktrees/hooks/use-run-start-dialog'
import { outcomeLabels, recapOf, startLabel } from '@/features/worktrees/run-start/utils/words'
import type { PhaseRun, Progress } from '@/features/worktrees/utils/progress'

const phaseWords = (restart: boolean): Record<PhaseRun['state'], string> => ({
  todo: 'queued',
  now: restart ? 'restarting' : 'starting',
  done: 'up',
  failed: 'failed',
})

// a start of one job is short and has no Cancel: a restart stopped half-way would leave the job down
export function RunStartProgress({
  progress,
  running,
  ran,
  onHide,
  onDone,
}: {
  progress: Progress<'run-start'>
  running: boolean
  ran: RunStartRan
  onHide: () => void
  onDone: () => void
}) {
  const envelope = progress.envelope
  const item = envelope?.items[0]
  const refused = progress.refused ?? (item?.error ? undefined : envelope?.error?.message)
  const title = startLabel(ran)

  const description = () => {
    if (running)
      return ran.restart ? `Restarting ${ran.job} in ${ran.worktree}…` : `Starting ${ran.job} in ${ran.worktree}…`
    if (!envelope?.ok) return ran.restart ? `${ran.job} didn’t restart.` : `${ran.job} didn’t start.`
    if (item?.detail?.jobs?.[0]?.status === 'done') return `${ran.job} ran.`
    return ran.restart ? `${ran.job} restarted.` : `${ran.job} started.`
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand />

      <DialogScroll>
        {running ? (
          progress.units.map((unit) => (
            <JobSection dense key={unit.subject} title={unit.subject}>
              {unit.phases.map((run) => (
                <PhaseRow
                  key={run.job}
                  name={run.job ?? ''}
                  run={run}
                  label={phaseWords(ran.restart)[run.state]}
                  lines={run.lines ?? []}
                />
              ))}
            </JobSection>
          ))
        ) : (
          <>
            <RunFailure message={refused} />
            <RunOutcome groups={recapOf(envelope)} labels={outcomeLabels} warnings={envelope?.warnings} />
          </>
        )}
      </DialogScroll>

      <DialogFooter className="items-center">
        {running ? (
          <>
            <p className="mr-auto text-sm text-muted-foreground">Closing this keeps it running.</p>
            <Button variant="ghost" onClick={onHide}>
              Hide
            </Button>
          </>
        ) : (
          <Button onClick={onDone}>Done</Button>
        )}
      </DialogFooter>
    </>
  )
}
