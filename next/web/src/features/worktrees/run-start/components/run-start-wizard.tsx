import { useQuery } from '@tanstack/react-query'
import { useReducer } from 'react'
import { dryRunQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainAnswered } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { NextButton } from '@/features/worktrees/components/next-button'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { Refusal } from '@/features/worktrees/components/refusal'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import type { RunStartAsk, RunStartRan, RunStartStep } from '@/features/worktrees/hooks/use-run-start-dialog'
import { ConfirmStep } from '@/features/worktrees/run-start/components/confirm-step'
import { JobStep } from '@/features/worktrees/run-start/components/job-step'
import { startLabel } from '@/features/worktrees/run-start/utils/words'
import { jobsTodo } from '@/features/worktrees/utils/progress'

const stepTitle: Record<RunStartStep, string> = { job: 'Job', confirm: 'Confirm' }

type State = { step: RunStartStep; job: string; force: boolean; exclusive: boolean }

type Action =
  | { type: 'go'; to: RunStartStep }
  | { type: 'pick'; job: string }
  | { type: 'consent'; to: 'force' | 'exclusive' }

// force and exclusive are the person's consent to a refusal, given for this job only
function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'go':
      return { ...state, step: action.to }
    case 'pick':
      return { ...state, job: action.job, force: false, exclusive: false }
    case 'consent':
      return { ...state, [action.to]: true }
  }
}

// opened on a job (a restart, a job named), it has no Job step
export function RunStartWizard({
  repo,
  ask,
  onStart,
  onCancel,
}: {
  repo: string
  ask: RunStartAsk
  onStart: (start: Start, ran: RunStartRan) => void
  onCancel: () => void
}) {
  const [{ step, job, force, exclusive }, dispatch] = useReducer(reducer, {
    step: ask.step,
    job: ask.job,
    force: false,
    exclusive: false,
  })
  const steps: RunStartStep[] = ask.step === 'job' ? ['job', 'confirm'] : ['confirm']

  const form = useQuery(questionsQuery(repo, 'run-start', [{ path: 'worktree', values: [ask.worktree] }]))
  const field = form.data?.fields.find((one) => one.path === 'job')
  const options = (field?.kind === 'select' ? field.options : []).filter((option) => !option.running_in?.length)

  const answers: DomainAnswered[] = [
    { path: 'worktree', values: [ask.worktree] },
    { path: 'job', values: [job] },
    // a restart puts back a job that ran beside the others' a moment ago: they are kept, as a job's direct restart
    ...(ask.restart
      ? [
          { path: 'restart', values: ['true'] },
          { path: 'parallel', values: ['true'] },
        ]
      : []),
    ...(force ? [{ path: 'force', values: ['true'] }] : []),
    ...(exclusive ? [{ path: 'exclusive', values: ['true'] }] : []),
  ]
  const planned = useQuery({ ...dryRunQuery(repo, 'run-start', answers), enabled: step === 'confirm' && job !== '' })
  const refusal = planned.data?.error ?? undefined
  const plan = refusal ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  const unplanned = planned.isError ? planned.error.message : refusal?.message
  const units = (plan?.worktrees ?? []).flatMap((target) => {
    const names = (target.jobs ?? []).filter((one) => one.action !== 'up').map((one) => one.name)
    return names.length > 0 ? [jobsTodo(target.worktree, names)] : []
  })

  const state = (of: RunStartStep): StepState => {
    if (of === step) return 'now'
    return steps.indexOf(of) < steps.indexOf(step) ? 'done' : 'todo'
  }

  const runBlocked = (): string | undefined => {
    if (unplanned) return 'The plan was refused'
    if (!plan || !fingerprint || planned.isFetching) return 'Planning…'
    if (units.length === 0) return `${job} runs already: nothing to start`
    return undefined
  }

  const run = () => {
    if (!plan || !fingerprint) return
    onStart({ answers, fingerprint, units }, { worktree: ask.worktree, job, restart: ask.restart, plan })
  }

  const body = () => {
    if (form.isError) return <RunFailure message={form.error.message} />
    if (step === 'job') {
      if (!form.data) return <Skeleton className="h-40 w-full" />
      return (
        <JobStep
          worktree={ask.worktree}
          options={options}
          value={job}
          onChange={(value) => dispatch({ type: 'pick', job: value })}
        />
      )
    }
    if (unplanned)
      return <Refusal refusal={refusal} message={unplanned} onConsent={(to) => dispatch({ type: 'consent', to })} />
    if (!plan) return <Skeleton className="h-40 w-full" />
    return <ConfirmStep plan={plan} warnings={planned.data?.warnings ?? []} />
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{ask.restart ? `Restart ${ask.job}` : 'Start one job'}</DialogTitle>
        <DialogDescription>
          {ask.restart
            ? `Stops ${ask.job} and starts it again, for every worktree holding it.`
            : `Starts one job in ${ask.worktree}.`}
        </DialogDescription>
      </DialogHeader>

      {steps.length > 1 && (
        <DialogBand>
          <Stepper steps={steps.map((one) => ({ title: stepTitle[one], state: state(one) }))} />
        </DialogBand>
      )}

      <DialogScroll>{body()}</DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {step === 'confirm' && steps.length > 1 && (
          <Button variant="outline" onClick={() => dispatch({ type: 'go', to: 'job' })}>
            Back
          </Button>
        )}
        {step === 'job' && (
          <NextButton
            disabled={job === ''}
            why={job === '' ? 'Pick a job' : undefined}
            onClick={() => dispatch({ type: 'go', to: 'confirm' })}
          />
        )}
        {step === 'confirm' && (
          <ReasonButton disabled={runBlocked() !== undefined} why={runBlocked()} onClick={run}>
            {startLabel({ job, restart: ask.restart })}
          </ReasonButton>
        )}
      </DialogFooter>
    </>
  )
}
