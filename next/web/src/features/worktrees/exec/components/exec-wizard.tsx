import { useDebouncedValue } from '@tanstack/react-pacer'
import { useQuery } from '@tanstack/react-query'
import { useReducer, useState } from 'react'
import { dryRunQuery, questionsQuery } from '@/api/queries/commands'
import { connectionOf, eventsQuery } from '@/api/queries/events'
import type { DomainAnswered } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { NextButton } from '@/features/worktrees/components/next-button'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import { CommandStep } from '@/features/worktrees/exec/components/command-step'
import { ConfirmStep, NothingLeft } from '@/features/worktrees/exec/components/confirm-step'
import { WorktreesStep } from '@/features/worktrees/exec/components/worktrees-step'
import { steps, wizardReducer } from '@/features/worktrees/exec/utils/wizard-state'
import { lockWords } from '@/features/worktrees/exec/utils/words'
import type { ExecAsk, Ran } from '@/features/worktrees/hooks/use-exec-dialog'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { todo } from '@/features/worktrees/utils/progress'

export function ExecWizard({
  repo,
  ask,
  onStart,
  onCancel,
}: {
  repo: string
  ask: ExecAsk
  onStart: (start: Start, ran: Ran) => void
  onCancel: () => void
}) {
  const [{ step, checked, command }, dispatch] = useReducer(wizardReducer, {
    step: ask.step,
    checked: ask.worktrees,
    command: ask.command,
  })

  const form = useQuery(questionsQuery(repo, 'exec'))
  const options = form.data?.fields.find((field) => field.path === 'worktree')
  const worktrees = options?.kind === 'multi_select' ? options.options.map((option) => option.value) : []
  // in the core's order, whatever order they were checked in: each step lists them alike. One checked that left the
  // options (removed meanwhile) is out of the request, and said so where the rows the plan leaves out are
  const chosen = form.data ? worktrees.filter((worktree) => checked.includes(worktree)) : checked
  const gone = form.data ? checked.filter((worktree) => !worktrees.includes(worktree)) : []
  const left = [...ask.left, ...gone.map((worktree) => ({ worktree, why: 'No longer exists' }))]

  // the shell's verdict once typing pauses: the core asks sh -n, the screen decides nothing of the line
  const [typed] = useDebouncedValue(command, { wait: 300 })
  const typedAnswers: DomainAnswered[] = [
    { path: 'worktree', values: chosen },
    { path: 'command', values: [typed] },
  ]
  const verdict = useQuery({
    ...questionsQuery(repo, 'exec', typedAnswers),
    enabled: step === 'command' && typed.trim() !== '',
  })
  const refused = verdict.data?.problems.find((problem) => problem.path === 'command')
  const problem = refused && (refused.params?.reason ?? refused.message)
  const checking = typed !== command || verdict.isFetching

  const answers: DomainAnswered[] = [
    { path: 'worktree', values: chosen },
    { path: 'command', values: [command] },
  ]
  const planned = useQuery({
    ...dryRunQuery(repo, 'exec', answers),
    enabled: step === 'confirm' && form.isSuccess && chosen.length > 0,
  })
  const plan = planned.data?.error ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  // shortcut: a plan refused as invalid reads as its one message, not its field errors; the Worktrees and Command steps
  // already rule out the gone worktree and the line sh refuses, so word the fields once a case is found that reaches here
  const unplanned = planned.isError ? planned.error.message : planned.data?.error?.message
  const targets = plan?.targets ?? []

  // a holder the event stream shows still under way still holds the lock; one it no longer shows may have let go
  const events = useQuery({ ...eventsQuery, select: (live) => live.operations })
  const operations = events.data ?? {}
  const holder = ask.held?.params?.op
  // a holder seen under way then gone has let go; one never seen (started before the page) may still hold it, and so
  // may one a reconnect lost: the stream starts again from nothing, so what it saw before no longer counts
  const streaming = connectionOf(events) === 'live'
  const holding = streaming && step === 'confirm' && holder !== undefined && Object.hasOwn(operations, holder)
  const [seen, setSeen] = useState(false)
  if (holding && !seen) setSeen(true)
  if (seen && !streaming) setSeen(false)
  const held = holding || !seen ? ask.held : undefined

  const state = (of: (typeof steps)[number]): StepState => {
    if (of === step) return 'now'
    return steps.indexOf(of) < steps.indexOf(step) ? 'done' : 'todo'
  }

  const blocked = (): string | undefined => {
    if (step === 'worktrees') return chosen.length === 0 ? 'Check at least one worktree' : undefined
    if (command.trim() === '') return 'Type a command first'
    if (problem) return 'Fix the command first'
    return checking ? 'Checking the command' : undefined
  }

  const runBlocked = (): string | undefined => {
    if (chosen.length === 0) return 'Check at least one worktree'
    if (holding && ask.held) return lockWords(ask.held, true).title
    if (unplanned) return 'The plan was refused'
    if (!plan || !fingerprint || planned.isFetching) return 'Planning…'
    return undefined
  }

  const next = () => dispatch({ type: 'go', to: step === 'worktrees' ? 'command' : 'confirm' })
  const back = () => dispatch({ type: 'go', to: step === 'confirm' ? 'command' : 'worktrees' })

  const run = () => {
    if (!plan || !fingerprint) return
    onStart(
      { answers, fingerprint, units: targets.map((target) => todo(target.worktree, ['exec'])) },
      { worktrees: chosen, command: plan.command, jobs: plan.jobs },
    )
  }

  const body = () => {
    if (form.isError) return <RunFailure message={form.error.message} />
    if (step === 'worktrees') {
      if (form.isPending) return <Skeleton className="h-40 w-full" />
      return (
        <WorktreesStep
          options={worktrees}
          checked={chosen}
          left={left}
          onCheck={(names, on) => dispatch({ type: 'check', worktrees: names, on })}
        />
      )
    }

    if (step === 'command')
      return (
        <CommandStep
          command={command}
          worktrees={chosen}
          problem={problem}
          onType={(typing) => dispatch({ type: 'type', command: typing })}
          onSubmit={() => !blocked() && next()}
        />
      )

    if (chosen.length === 0) return <NothingLeft left={left} />
    if (unplanned) return <RunFailure message={unplanned} />
    if (!plan) return <Skeleton className="h-40 w-full" />
    return <ConfirmStep plan={plan} warnings={planned.data?.warnings ?? []} left={left} held={held} live={holding} />
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Run a command</DialogTitle>
        <DialogDescription>It runs in each worktree's folder, several at once.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Worktrees', state: state('worktrees') },
            { title: 'Command', state: state('command') },
            { title: 'Confirm', state: state('confirm') },
          ]}
        />
      </DialogBand>

      <DialogScroll>{body()}</DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {step !== 'worktrees' && (
          <Button variant="outline" onClick={back}>
            Back
          </Button>
        )}
        {step !== 'confirm' && <NextButton disabled={blocked() !== undefined} why={blocked()} onClick={next} />}
        {step === 'confirm' && (
          <ReasonButton disabled={runBlocked() !== undefined} why={runBlocked()} onClick={run}>
            {chosen.length === 1 ? 'Run' : `Run in ${chosen.length} worktrees`}
          </ReasonButton>
        )}
      </DialogFooter>
    </>
  )
}
