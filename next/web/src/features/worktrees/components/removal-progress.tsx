import { useQuery } from '@tanstack/react-query'
import { Database } from 'lucide-react'
import { readQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunSteps } from '@/features/worktrees/components/run-steps'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { useReparentNow } from '@/features/worktrees/hooks/use-reparent-now'
import { useRun } from '@/features/worktrees/hooks/use-run'
import { listed } from '@/features/worktrees/utils/listed'
import type { Act, OutcomeKind } from '@/features/worktrees/utils/outcome'
import { phaseLabel } from '@/features/worktrees/utils/phases'
import { current, doneOf, ending, type Progress } from '@/features/worktrees/utils/progress'
import { plain } from '@/features/worktrees/utils/removal'
import { recapOf, type Teardown } from '@/features/worktrees/utils/removal-recap'

// a command that removes worktrees says itself in these: its dialog's title, what became of each, its run
export type RemovalWords = {
  title: string
  past: string
  noun: string
  outcome: Record<OutcomeKind, string>
  dataKept: string
}

export function RemovalProgress({
  repo,
  words,
  progress,
  running,
  onHide,
  onCancel,
  onDone,
}: {
  repo: string
  words: RemovalWords
  progress: Progress<Teardown>
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
  // the run's own failure (a stale plan, a refusal); one the core repeats from an item shows on that item's line, and a
  // cancel is said by the items it stopped
  const error = progress.envelope?.error
  const refused =
    progress.refused ?? (items.some((item) => item.error) || error?.kind === 'cancelled' ? undefined : error?.message)
  const { groups, warnings, kept } = recapOf(progress, words.dataKept)
  const reparentNow = useReparentNow(repo)
  const reparenting = useRun(repo, 'reparent').running
  const act: Act = {
    command: 'reparent',
    label: 'Reparent',
    all: 'Reparent all onto the nearest that stays',
    busy: reparenting,
    run: (followUps) => void reparentNow(followUps),
  }
  // never asked here: the view on screen reads, and a child moved leaves "To check" once its read lands
  const list = useQuery({ ...readQuery(repo, 'list'), enabled: false })
  const tree = useQuery({ ...readQuery(repo, 'tree'), enabled: false })
  const parentNow = (branch: string) =>
    [...(list.data?.items ?? []), ...(tree.data?.items ?? [])].find((item) => item.subject === branch)?.detail?.parent
  const moved = ({ params }: (typeof warnings)[number]) =>
    params?.branch !== undefined && params.to !== undefined && parentNow(params.branch) === params.to
  const toCheck = warnings.filter((warning) => !moved(warning))
  const branched = plain(
    (progress.envelope?.plan?.worktrees ?? []).filter((removal) => removed.includes(removal.worktree)),
  )

  const description = () => {
    if (ok && !branched) return `${removed.length} ${words.past}.`
    if (ok)
      return names.length === 1
        ? `1 ${words.past}. Its local branch went with it; the remote branch stays.`
        : `${removed.length} ${words.past}. Their local branches went with them; the remote branches stay.`
    if (!running)
      return `${removed.length} of ${names.length} ${words.past}.${cancelled ? ' Those not started stay.' : ''}`
    if (progress.cancelling) return `Cancelling the ${words.noun} of ${listed(failed)}`
    if (now) return `Step ${now.index + 1} of ${now.total} · ${phaseLabel[now.run.phase]} of ${now.subject}`
    return done > 0 ? 'Finishing…' : 'Starting…'
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{words.title}</DialogTitle>
        <DialogDescription>{description()}</DialogDescription>
      </DialogHeader>
      <DialogBand>{running && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}</DialogBand>

      <DialogScroll>
        {!running && <RunFailure message={refused} />}
        {!running && <RunOutcome groups={groups} labels={words.outcome} warnings={toCheck} act={act} />}
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
            stopping={`Finishing the ${words.noun} under way, then it stops.`}
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
