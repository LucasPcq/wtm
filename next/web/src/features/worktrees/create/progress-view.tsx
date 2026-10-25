import { Check, ChevronRight, CircleCheck, CircleX, Copy, LoaderCircle, Square, X } from 'lucide-react'
import type { Applied } from '@/api/queries/commands'
import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Progress as Bar } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import { InlineCommand } from '../shared/command-line'
import { sentence } from '../shared/sentence'
import { copy } from '../shared/copy'
import { phaseLabel } from './phases'
import { current, doneOf, seconds, took, type PhaseRun, type Progress } from './progress'

// ProgressView is a create under way, then how it ended: each phase of each branch done, under way or to come, the
// output of the hook that runs, and a cancel, then a forced stop. A run that went well ends on each worktree and what
// to do with it, its steps folded away; a failed or cancelled one keeps them open, to show where it stopped.
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
  const cancelled = progress.envelope?.items?.some((item) => item.status === 'cancelled') ?? false
  const failure = cancelled ? undefined : (progress.refused ?? failureOf(progress.envelope))
  const ok = ended && !failure && !cancelled
  const succeeded = progress.units.flatMap(([, unit]) => unit.phases).filter((run) => run.state === 'done').length
  const names = branches.join(', ')
  const title = ended
    ? cancelled
      ? `Cancelled ${names}`
      : failure
        ? `Couldn't create ${names}`
        : `${names} ${branches.length === 1 ? 'is' : 'are'} ready`
    : progress.cancelling
      ? `Cancelling ${names}`
      : `Creating ${names}`
  const summary = [`${total} steps`, took(progress)].filter(Boolean).join(' · ')
  const description = ended
    ? `${succeeded} of ${total} steps done.${cancelled ? ' What is done stays: clean it to remove it.' : ''}`
    : now
      ? `Step ${now.index + 1} of ${now.total} · ${phaseLabel[now.run.phase]}`
      : done > 0
        ? 'Finishing…'
        : 'Starting…'

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription className={cn(ok && 'sr-only')}>{description}</DialogDescription>
      </DialogHeader>
      {!ended && <Bar value={total ? (done / total) * 100 : 0} aria-label="Progress" />}

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
                  {summary}
                </Button>
              </CollapsibleTrigger>
              <CollapsibleContent className="pt-3">
                <Timeline progress={progress} />
              </CollapsibleContent>
            </Collapsible>
          ) : (
            <Timeline progress={progress} />
          )}
          {ended && failure && <p className="text-sm text-destructive">{failure}</p>}
          {ended && <Made envelope={progress.envelope} />}
        </div>
      </ScrollArea>

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

// Timeline is each branch's phases, done, under way or to come, and the output of the hook that runs or failed.
function Timeline({ progress }: { progress: Progress }) {
  return (
    <div className="flex flex-col gap-4">
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
                      <pre
                        // the output follows its last line, as a terminal does
                        ref={(pre) => pre?.scrollTo({ top: pre.scrollHeight })}
                        className="h-28 overflow-y-auto font-mono text-xs whitespace-pre-wrap text-muted-foreground [scrollbar-color:var(--border)_transparent] [scrollbar-width:thin]"
                      >
                        {unit.lines.join('\n')}
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
    </div>
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

function failureOf(envelope: Applied<'create'> | undefined): string | undefined {
  if (!envelope) return undefined
  return envelope.error?.message ?? envelope.items?.find((item) => item.error)?.error?.message
}

// Made is what a create that ended leaves, as its envelope says: each worktree, where it is and the command to go on
// with it, then a follow-up of none in particular; each a command to copy until the web runs it.
function Made({ envelope }: { envelope: Applied<'create'> | undefined }) {
  if (!envelope) return null
  const items = envelope.items ?? []
  const nextOf = (subject: string) =>
    envelope.follow_ups.find((followUp) =>
      followUp.answers?.some((answer) => answer.path === 'worktree' && answer.values[0] === subject),
    )
  const loose = envelope.follow_ups.filter((followUp) => !items.some((item) => nextOf(item.subject) === followUp))
  return (
    <div className="flex flex-col gap-3 not-first:border-t not-first:pt-4">
      {items.map((item) =>
        item.status === 'done' ? (
          <Worktree key={item.subject} branch={item.subject} path={item.detail.path} next={nextOf(item.subject)} />
        ) : (
          <p key={item.subject} className="flex items-center gap-2 text-sm">
            <CircleX className="size-4 shrink-0 text-destructive" aria-hidden="true" />
            <span className="font-medium">{item.subject}</span>
            <span className="text-muted-foreground">{item.status}</span>
          </p>
        ),
      )}
      {loose.map((followUp) => (
        <p key={followUp.code + JSON.stringify(followUp.answers)} className="flex items-center gap-3 text-sm">
          <span className="flex-1">{sentence(followUp.message)}</span>
          <InlineCommand followUp={followUp} />
        </p>
      ))}
    </div>
  )
}

function Worktree(props: { branch: string; path: string; next: DomainFollowUp | undefined }) {
  return (
    <div className="flex items-center gap-3">
      <CircleCheck className="size-4 shrink-0 text-green-500" aria-hidden="true" />
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="text-sm font-medium">{props.branch}</span>
        <button
          type="button"
          title={props.path}
          className="flex max-w-full min-w-0 items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground"
          onClick={() => copy(props.path)}
        >
          <span className="truncate [direction:rtl]">
            <bdi>{props.path}</bdi>
          </span>
          <Copy className="size-3 shrink-0" aria-label="Copy the path" />
        </button>
      </div>
      {props.next && <InlineCommand followUp={props.next} />}
    </div>
  )
}
