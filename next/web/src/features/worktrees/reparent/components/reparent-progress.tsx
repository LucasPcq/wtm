import type { DomainAnswered } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { RunOutcome } from '@/features/worktrees/components/run-outcome'
import { RunningFooter } from '@/features/worktrees/components/running-footer'
import { RunningUnits, type UnitState } from '@/features/worktrees/components/running-units'
import { moved, syncOf } from '@/features/worktrees/reparent/utils/plan'
import { failureOf, moveWords, outcomeWords, title } from '@/features/worktrees/reparent/utils/words'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'
import { ending, type Progress } from '@/features/worktrees/utils/progress'

const stateWords: Record<UnitState, string> = {
  todo: 'waiting',
  now: 'recording its new parent',
  done: 'reparented',
  failed: 'failed',
}

export function ReparentProgress({
  progress,
  running,
  onHide,
  onCancel,
  onDone,
  onSync,
}: {
  progress: Progress<'reparent'>
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onDone: () => void
  onSync: (answers: DomainAnswered[]) => void
}) {
  const envelope = progress.envelope
  const items = envelope?.items ?? []
  const moves = envelope?.plan?.moves ?? []
  const went = moved(envelope ? [envelope] : [])
  const done = progress.units.filter((unit) => unit.status).length
  const total = progress.units.length
  const { cancelled, failure } = ending(progress)
  const refused = progress.refused ?? (items.some((item) => item.error) ? undefined : envelope?.error?.message)
  const sync = syncOf(envelope ? [envelope] : [])
  const [parent] = [...new Set(went.flatMap((item) => (item.detail ? [item.detail.to] : [])))]

  // the writes go one after the other: the first not finished is the one under way
  const stateOf = (index: number): UnitState => {
    const status = progress.units[index]?.status
    if (status) return status === 'done' ? 'done' : 'failed'
    return progress.units.findIndex((unit) => !unit.status) === index ? 'now' : 'todo'
  }
  const moveOf = (subject: string) => moves.find((move) => move.branch === subject)

  const description = () => {
    if (running && progress.cancelling) return 'Cancelling: the worktree under way finishes first.'
    if (running) return total === 0 ? 'Planning' : `${done} of ${total}`
    if (cancelled) return 'Cancelled: what is done stays.'
    if (failure) return `${went.length} of ${progress.units.length} reparented.`
    if (went.length === 1) return `1 reparented. It rebases onto ${parent} on its next sync.`
    return `${went.length} reparented. Each rebases onto ${parent} on its next sync.`
  }

  const doneRow = (subject: string): OutcomeRow => {
    const item = items.find((one) => one.subject === subject)
    const move = moveOf(subject)
    if (item?.reason === 'unchanged')
      return { key: subject, tone: 'muted', title: subject, message: `already on ${item.detail?.to}` }
    return { key: subject, tone: 'ok', title: subject, message: move && moveWords(move), path: item?.detail?.path }
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
          <RunningUnits
            rows={progress.units.map(({ subject }, i) => {
              const state = stateOf(i)
              const move = moveOf(subject)
              return { subject, state, words: state === 'done' && move ? moveWords(move) : stateWords[state] }
            })}
          />
        ) : (
          <>
            {!cancelled && <RunFailure message={refused} />}
            <RunOutcome
              groups={[
                {
                  kind: 'failed',
                  rows: items
                    .filter((item) => item.status === 'failed' || item.reason === 'interrupted')
                    .map((item) =>
                      item.status === 'failed'
                        ? {
                            key: item.subject,
                            tone: 'failed',
                            title: item.subject,
                            message: failureOf(item.error),
                            output: item.error
                              ? { lines: [item.error.params?.cause ?? item.error.message] }
                              : undefined,
                          }
                        : {
                            key: item.subject,
                            tone: 'muted',
                            title: item.subject,
                            message: 'not reached: cancelled first',
                          },
                    ),
                },
                {
                  kind: 'done',
                  // what moved first, then what was already there
                  rows: [
                    ...items.filter((item) => item.status === 'done' && item.reason !== 'unchanged'),
                    ...items.filter((item) => item.reason === 'unchanged'),
                  ].map((item) => doneRow(item.subject)),
                },
              ]}
              labels={outcomeWords}
              warnings={envelope?.warnings}
            />
            {sync && (
              <section aria-label="Next" className="flex flex-col gap-2">
                <h3 className="text-xs font-medium text-muted-foreground">Next</h3>
                <Button variant="outline" className="w-fit" onClick={() => onSync(sync.answers ?? [])}>
                  {went.length === 1 ? 'Sync it' : 'Sync them'}
                </Button>
              </section>
            )}
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
