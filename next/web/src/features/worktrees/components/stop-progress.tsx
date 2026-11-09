import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { JobSection } from '@/features/worktrees/components/job-section'
import { PhaseRow } from '@/features/worktrees/components/phase-row'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import type { RunDownRan } from '@/features/worktrees/hooks/use-run-down-dialog'
import type { PhaseRun, Progress } from '@/features/worktrees/utils/progress'
import { where } from '@/features/worktrees/utils/run-words'
import { countOf, holdsOn, sharedOf, stopJob } from '@/features/worktrees/utils/stop-plan'
import { recapOf } from '@/features/worktrees/utils/stop-recap'
import { sharedStopping, stopping } from '@/features/worktrees/utils/stop-words'

// a shared service is let go by each of its holders: its row says where the least advanced one stands
const rank: Record<PhaseRun['state'], number> = { failed: 0, now: 1, todo: 2, done: 3 }

// a stop is never cancelled: a stack stopped half-way is worse than one stopped or not
export function StopProgress({
  title,
  stopping: doing,
  progress,
  running,
  ran,
  onHide,
  onDone,
  onAgain,
  ended,
}: {
  title: string
  stopping: string
  progress: Progress<'run-down' | 'daemon-stop-jobs'>
  running: boolean
  ran: RunDownRan
  onHide: () => void
  onDone: () => void
  onAgain?: (() => void) | undefined
  // the outcome said by the command's own words: the jobs stopped of all, and the ones that did not stop
  ended?: ((count: { stopped: number; total: number }, failed: string[]) => string) | undefined
}) {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const { done, stopped, total } = countOf(progress.units, ran.plan)
  const failed = items.filter((item) => item.status !== 'done').length
  const refused = progress.refused ?? (items.some((item) => item.error) ? undefined : envelope?.error?.message)
  const { groups, labels, warnings } = recapOf(envelope, ran.plan, progress.units)
  const shared = sharedOf(ran.plan)
  // by the plan's job of that worktree: a job of its own may be named like a shared service
  const isShared = (worktree: string, name: string | undefined) =>
    Boolean(stopJob(ran.plan, worktree, name ?? '')?.shared_in)

  const sharedRows = shared.flatMap((service) => {
    const runs = progress.units.flatMap((unit) =>
      unit.phases.filter((run) => holdsOn(ran.plan, unit.subject, run.job, service)),
    )
    const least = runs.toSorted((a, b) => rank[a.state] - rank[b.state])[0]
    return least ? [{ service, run: least, lines: runs.flatMap((run) => run.lines ?? []) }] : []
  })

  const description = () => {
    if (running) return `${doing} · ${done} of ${total} stopped`
    if (items.length === 0) return 'Nothing stopped.'
    if (ended) {
      const notStopped = progress.units.flatMap((unit) =>
        unit.phases.filter((run) => run.state === 'failed').map((run) => `${run.job} in ${unit.subject}`),
      )
      return ended({ stopped, total }, notStopped)
    }
    if (failed > 0) return `Jobs stopped in ${items.length - failed} of ${items.length} worktrees.`
    return `Jobs stopped ${where(items.map((item) => item.subject))}.`
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {running ? (
          <>
            {progress.units.map((unit) => (
              <JobSection dense key={unit.subject} title={unit.subject}>
                {unit.phases
                  .filter((run) => !isShared(unit.subject, run.job))
                  .map((run) => (
                    <PhaseRow
                      key={run.job}
                      name={run.job ?? ''}
                      run={run}
                      label={stopping(run, stopJob(ran.plan, unit.subject, run.job ?? ''))}
                      lines={run.lines ?? []}
                    />
                  ))}
              </JobSection>
            ))}
            {sharedRows.length > 0 && (
              <JobSection dense title="Shared services">
                {sharedRows.map(({ service, run, lines }) => (
                  <PhaseRow
                    key={`${service.sharedIn}:${service.name}`}
                    name={service.name}
                    run={run}
                    label={sharedStopping(
                      run,
                      service,
                      items.flatMap((item) =>
                        (item.detail?.jobs ?? [])
                          .filter((job) => holdsOn(ran.plan, item.subject, job.name, service))
                          .map((job) => job.status),
                      ),
                    )}
                    lines={lines}
                  />
                ))}
              </JobSection>
            )}
          </>
        ) : (
          <>
            <RunFailure message={refused} />
            <RunOutcome groups={groups} labels={labels} warnings={warnings} />
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
          <>
            {onAgain && failed > 0 && (
              <Button variant="outline" onClick={onAgain}>
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
