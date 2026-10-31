import type { ReactNode } from 'react'
import type { RunupJobPlan } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import type { RunUpRan } from '@/features/worktrees/hooks/use-run-up-dialog'
import { JobRow } from '@/features/worktrees/run-up/components/job-row'
import { recapOf } from '@/features/worktrees/run-up/utils/recap'
import { startedWords, underWay, where } from '@/features/worktrees/run-up/utils/words'
import { doneOf, ending, type PhaseRun, type Progress } from '@/features/worktrees/utils/progress'

// a shared service is joined by each worktree: its row says where the least advanced one stands
const rank: Record<PhaseRun['state'], number> = { failed: 0, now: 1, todo: 2, done: 3 }

export function RunUpProgress({
  progress,
  running,
  ran,
  onHide,
  onCancel,
  onDone,
}: {
  progress: Progress<'run-up'>
  running: boolean
  ran: RunUpRan
  onHide: () => void
  onCancel: (force: boolean) => void
  onDone: () => void
}) {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const { done, total } = doneOf(progress)
  const { cancelled } = ending(progress)
  const failed = items.filter((item) => item.status === 'failed').length
  const refused =
    progress.refused ?? (cancelled || items.some((item) => item.error) ? undefined : envelope?.error?.message)
  const { groups, labels, warnings } = recapOf(envelope)
  const cancelling = progress.cancelling !== undefined

  const planOf = (worktree: string, name: string): RunupJobPlan | undefined =>
    ran.plan?.worktrees?.find((target) => target.worktree === worktree)?.jobs?.find((job) => job.name === name)
  const joined = (worktree: string, name: string) => planOf(worktree, name)?.action === 'join'

  const shared = [
    ...new Set(
      progress.units.flatMap((unit) =>
        unit.phases.flatMap((run) => (run.job && joined(unit.subject, run.job) ? [run.job] : [])),
      ),
    ),
  ].flatMap((name) => {
    const runs = progress.units.flatMap((unit) =>
      unit.phases.filter((run) => run.job === name).map((run) => ({ run, job: planOf(unit.subject, name) })),
    )
    const least = runs.toSorted((a, b) => rank[a.run.state] - rank[b.run.state])[0]
    return least ? [{ name, ...least }] : []
  })

  const description = () => {
    if (running && progress.cancelling) return `Cancelling · ${done} of ${total} started`
    if (running) return `Starting ${ran.profile} ${where(ran.worktrees)} · ${done} of ${total} started`
    if (items.length === 0) return 'Nothing started.'
    if (cancelled) return 'Cancelled: what started keeps running.'
    if (failed > 0) return `${ran.profile} didn't start in ${failed} of ${items.length}.`
    return `${startedWords(ran.profile, envelope).title}.`
  }

  const stopping = () => {
    const queued = progress.units.flatMap((unit) => unit.phases).filter((run) => run.state === 'todo').length
    const wont = queued === 1 ? "the queued job won't start" : `the ${queued} queued jobs won't start`
    return queued === 0
      ? 'Stopping the start under way. What started keeps running.'
      : `Stopping the start under way; ${wont}.`
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Start a profile</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {running ? (
          <>
            {shared.length > 0 && (
              <Group title="Shared services">
                {shared.map(({ name, run, job }) => (
                  <JobRow key={name} name={name} run={run} said={underWay(run, job, cancelling)} />
                ))}
              </Group>
            )}
            {progress.units.map((unit) => (
              <Group key={unit.subject} title={unit.subject}>
                {unit.phases
                  .filter((run) => !(run.job && joined(unit.subject, run.job)))
                  .map((run) => (
                    <JobRow
                      key={run.job}
                      name={run.job ?? ''}
                      run={run}
                      said={underWay(run, planOf(unit.subject, run.job ?? ''), cancelling)}
                    />
                  ))}
              </Group>
            ))}
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
          <RunningFooter progress={progress} stopping={stopping()} onHide={onHide} onCancel={onCancel} />
        ) : (
          <Button onClick={onDone}>Done</Button>
        )}
      </DialogFooter>
    </>
  )
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-1">
      <h3 className="text-xs font-medium text-muted-foreground">{title}</h3>
      <ul className="flex flex-col gap-1">{children}</ul>
    </section>
  )
}
