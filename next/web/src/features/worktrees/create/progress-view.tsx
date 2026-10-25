import { Check, CircleCheck, CircleX, LoaderCircle, Square, X } from 'lucide-react'
import type { CreateDetail, DomainEnvelope } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { cn } from '@/lib/utils'
import { CommandLine } from '../shared/command-line'
import { copy } from '../shared/copy'
import { phaseLabel } from './phases'
import { current, doneOf, seconds, type PhaseRun, type Progress } from './progress'

// ProgressView is a create under way, then how it ended: each phase of each branch done, under way or to come, the
// output of the hook that runs, and a cancel, then a forced stop.
export function ProgressView(props: {
  progress: Progress
  running: boolean
  onHide: () => void
  onCancel: (force: boolean) => void
  onAnother: () => void
  onDone: (branch: string | undefined) => void
}) {
  const { progress } = props
  const branches = progress.units
    .filter(([, unit]) => unit.phases.some((run) => run.phase !== 'fast_forward'))
    .map(([subject]) => subject)
  const now = current(progress)
  const { done, total } = doneOf(progress)
  const ended = !props.running
  const failure = progress.refused ?? failureOf(progress.envelope)
  const title = ended
    ? failure
      ? `Couldn't create ${branches.join(', ')}`
      : progress.envelope?.items?.some((item) => item.status === 'cancelled')
        ? `Cancelled ${branches.join(', ')}`
        : `${branches.join(', ')} ${branches.length === 1 ? 'is' : 'are'} ready`
    : progress.cancelling
      ? `Cancelling ${branches.join(', ')}`
      : `Creating ${branches.join(', ')}`

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription>
          {ended
            ? `${done} of ${total} steps done.`
            : now
              ? `Step ${now.index + 1} of ${now.total} · ${phaseLabel[now.run.phase]}`
              : 'Starting…'}
        </DialogDescription>
      </DialogHeader>
      {!ended && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}

      <div className="flex max-h-[50vh] flex-col gap-4 overflow-y-auto">
        {progress.units.map(([subject, unit]) => (
          <section key={subject} aria-label={subject}>
            {progress.units.length > 1 && <h3 className="mb-2 font-mono text-xs text-muted-foreground">{subject}</h3>}
            <ol className="flex flex-col">
              {unit.phases.map((run, i) => (
                <li key={run.phase} className="flex gap-3">
                  <div className="flex w-5 flex-col items-center">
                    <Marker run={run} />
                    {i < unit.phases.length - 1 && <span className="w-px flex-1 bg-border" />}
                  </div>
                  <div className="flex min-w-0 flex-1 flex-col gap-2 pb-3">
                    <div className="flex items-center text-sm">
                      <span
                        className={cn(
                          'flex-1',
                          run.state === 'todo' && 'text-muted-foreground',
                          run.state === 'now' && 'font-medium',
                        )}
                      >
                        {phaseLabel[run.phase]}
                      </span>
                      <span className="text-xs text-muted-foreground tabular-nums">{seconds(run)}</span>
                    </div>
                    {(run.state === 'now' || run.state === 'failed') && unit.lines.length > 0 && (
                      <div className="rounded-md bg-muted px-3 py-2">
                        <pre className="max-h-40 overflow-y-auto font-mono text-xs whitespace-pre-wrap text-muted-foreground">
                          {unit.lines.slice(-8).join('\n')}
                        </pre>
                        {run.log && (
                          <button
                            type="button"
                            className="mt-1 text-xs underline underline-offset-4"
                            onClick={() => run.log && copy(run.log)}
                          >
                            Copy the log's path
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                </li>
              ))}
            </ol>
          </section>
        ))}
        {ended && failure && <p className="text-sm text-destructive">{failure}</p>}
        {ended && <Ended envelope={progress.envelope} />}
      </div>

      <DialogFooter className="items-center">
        {!ended && (
          <>
            <p className="mr-auto text-sm text-muted-foreground">
              {progress.cancelling
                ? 'Finishing the step under way, then it stops. What is done stays.'
                : 'Closing this keeps it running.'}
            </p>
            <Button variant="ghost" onClick={props.onHide}>
              Hide
            </Button>
            {progress.cancelling ? (
              <Button
                variant="destructive"
                disabled={progress.cancelling === 'forced'}
                onClick={() => props.onCancel(true)}
              >
                Stop now
              </Button>
            ) : (
              <Button variant="outline" disabled={!progress.operation} onClick={() => props.onCancel(false)}>
                <Square />
                Cancel
              </Button>
            )}
          </>
        )}
        {ended && (
          <>
            <Button variant="outline" className="mr-auto" onClick={props.onAnother}>
              Create another
            </Button>
            <Button onClick={() => props.onDone(failure ? undefined : branches[0])}>Done</Button>
          </>
        )}
      </DialogFooter>
    </>
  )
}

function Marker({ run }: { run: PhaseRun }) {
  const look = {
    todo: <span className="size-1.5 rounded-full bg-muted-foreground" />,
    now: <LoaderCircle className="size-3 animate-spin motion-reduce:animate-none" />,
    done: <Check className="size-3 text-green-500" />,
    failed: <X className="size-3 text-destructive" />,
  }[run.state]
  return (
    <span
      className={cn(
        'flex size-5 shrink-0 items-center justify-center rounded-full',
        run.state === 'todo' ? 'border' : 'bg-muted',
      )}
    >
      {look}
    </span>
  )
}

function failureOf(envelope: DomainEnvelope | undefined): string | undefined {
  if (!envelope) return undefined
  return envelope.error?.message ?? envelope.items?.find((item) => item.error)?.error?.message
}

// Ended is what a create that ended leaves to do, as its envelope says: where each worktree is, then what to run
// next, each as a command line until the web runs it.
function Ended({ envelope }: { envelope: DomainEnvelope | undefined }) {
  if (!envelope) return null
  const made = (envelope.items ?? []).filter((item) => item.status === 'done')
  return (
    <div className="flex flex-col gap-3 border-t pt-4">
      {made.map((item) => {
        const detail = item.detail as unknown as CreateDetail
        return (
          <p key={item.subject} className="flex items-center gap-2 text-sm">
            <CircleCheck className="size-4 shrink-0 text-green-500" aria-hidden="true" />
            <span className="font-medium">{item.subject}</span>
            <span className="truncate font-mono text-xs text-muted-foreground">{detail.path}</span>
          </p>
        )
      })}
      {(envelope.items ?? [])
        .filter((item) => item.status !== 'done')
        .map((item) => (
          <p key={item.subject} className="flex items-center gap-2 text-sm">
            <CircleX className="size-4 shrink-0 text-destructive" aria-hidden="true" />
            <span className="font-medium">{item.subject}</span>
            <span className="text-muted-foreground">{item.status}</span>
          </p>
        ))}
      {envelope.follow_ups.length > 0 && (
        <div className="flex flex-col gap-2">
          <h3 className="text-sm font-semibold">Next</h3>
          <p className="text-sm text-muted-foreground">Not in the web interface yet: copy into a terminal.</p>
          {envelope.follow_ups.map((followUp) => (
            <div key={followUp.code + JSON.stringify(followUp.answers)} className="flex flex-col gap-1">
              <span className="text-sm">{followUp.message}</span>
              <CommandLine followUp={followUp} />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
