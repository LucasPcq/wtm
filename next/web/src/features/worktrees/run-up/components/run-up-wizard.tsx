import { useQuery } from '@tanstack/react-query'
import { useReducer } from 'react'
import { dryRunQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainAnswered, DomainField } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { NextButton } from '@/features/worktrees/components/next-button'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import type { RunUpAsk, RunUpRan, RunUpStep } from '@/features/worktrees/hooks/use-run-up-dialog'
import { ConfirmStep } from '@/features/worktrees/run-up/components/confirm-step'
import { ProfileStep } from '@/features/worktrees/run-up/components/profile-step'
import { Refusal } from '@/features/worktrees/run-up/components/refusal'
import { WorktreesStep } from '@/features/worktrees/run-up/components/worktrees-step'
import { unitsOf } from '@/features/worktrees/run-up/utils/plan'
import { steps, wizardReducer } from '@/features/worktrees/run-up/utils/wizard-state'
import { where } from '@/features/worktrees/run-up/utils/words'

const optionsOf = (fields: DomainField[] | undefined, path: string) => {
  const field = fields?.find((one) => one.path === path)
  return field?.kind === 'multi_select' || field?.kind === 'select' ? field : undefined
}

const stepTitle: Record<RunUpStep, string> = { worktrees: 'Worktrees', profile: 'Profile', confirm: 'Confirm' }

export function RunUpWizard({
  repo,
  ask,
  onStart,
  onCancel,
}: {
  repo: string
  ask: RunUpAsk
  onStart: (start: Start, ran: RunUpRan) => void
  onCancel: () => void
}) {
  const [{ step, checked, profile, force, exclusive }, dispatch] = useReducer(wizardReducer, {
    step: ask.step,
    checked: ask.worktrees,
    profile: ask.profile,
    force: false,
    exclusive: false,
  })

  const all = useQuery(questionsQuery(repo, 'run-up'))
  const worktreeOptions = optionsOf(all.data?.fields, 'worktree')?.options ?? []
  // in the core's order, whatever order they were checked in
  const chosen = all.data ? worktreeOptions.map(({ value }) => value).filter((name) => checked.includes(name)) : checked

  // the profiles are asked for the worktrees chosen: each says which of them run it already
  const form = useQuery({
    ...questionsQuery(repo, 'run-up', [{ path: 'worktree', values: chosen }]),
    enabled: chosen.length > 0,
  })
  const profileField = optionsOf(form.data?.fields, 'profile')
  // a run.toml of one profile, or of none, asks no profile: the core starts it, or every job
  const skipped = optionsOf(all.data?.fields, 'profile')?.skip !== undefined
  const shownSteps = skipped ? steps.filter((one) => one !== 'profile') : steps
  const profiles = profileField?.options ?? []
  const picked = profile || (profileField?.kind === 'select' ? profileField.value : '')
  // a worktree that runs the profile already is left as it is, as the core says it runs it
  const runsIt = profiles.find((one) => one.value === picked)?.running_in ?? []
  const starting = chosen.filter((worktree) => !runsIt.includes(worktree))
  const left = [
    ...chosen
      .filter((worktree) => runsIt.includes(worktree))
      .map((worktree) => ({ worktree, why: `already runs ${picked}` })),
    ...ask.left,
  ]

  // opened on Confirm with no profile the core can pick (no default among several), or with the default one every
  // worktree runs already: the profile is asked first, the default kept as the answer so that Next goes on
  const defaulted = ask.step === 'confirm' && step === 'confirm' && form.isSuccess && profile === '' && !skipped
  if (defaulted && (picked === '' || starting.length === 0)) {
    dispatch({ type: 'pick', profile: picked })
    dispatch({ type: 'go', to: 'profile' })
  }

  const answers: DomainAnswered[] = [
    { path: 'worktree', values: starting },
    ...(skipped || picked === '' ? [] : [{ path: 'profile', values: [picked] }]),
    ...(force ? [{ path: 'force', values: ['true'] }] : []),
    ...(exclusive ? [{ path: 'exclusive', values: ['true'] }] : []),
  ]
  const planned = useQuery({
    ...dryRunQuery(repo, 'run-up', answers),
    enabled: step === 'confirm' && form.isSuccess && starting.length > 0 && (skipped || picked !== ''),
  })
  const plan = planned.data?.error ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  const refusal = planned.data?.error ?? undefined
  const unplanned = planned.isError ? planned.error.message : refusal?.message
  const others = worktreeOptions
    .filter(({ value, up }) => !chosen.includes(value) && (up?.length ?? 0) > 0)
    .map(({ value }) => value)

  const state = (of: RunUpStep): StepState => {
    if (of === step) return 'now'
    return shownSteps.indexOf(of) < shownSteps.indexOf(step) ? 'done' : 'todo'
  }

  const blocked = (): string | undefined => {
    if (step === 'worktrees') return chosen.length === 0 ? 'Check at least one worktree' : undefined
    return picked === '' ? 'Pick a profile' : undefined
  }

  const runBlocked = (): string | undefined => {
    if (starting.length === 0) return `${picked} runs there already: nothing to start`
    if (unplanned) return 'The plan was refused'
    if (!plan || !fingerprint || planned.isFetching) return 'Planning…'
    if (plan.empty) return `${plan.profile} runs there already: nothing to start`
    return undefined
  }

  const next = () => {
    if (step === 'profile') dispatch({ type: 'pick', profile: picked })
    dispatch({ type: 'go', to: shownSteps[shownSteps.indexOf(step) + 1] ?? 'confirm' })
  }
  const back = () => dispatch({ type: 'go', to: shownSteps[shownSteps.indexOf(step) - 1] ?? 'worktrees' })

  const run = () => {
    if (!plan || !fingerprint) return
    onStart({ answers, fingerprint, units: unitsOf(plan) }, { worktrees: starting, profile: plan.profile, plan })
  }

  const body = () => {
    if (all.isError) return <RunFailure message={all.error.message} />
    if (step === 'worktrees') {
      if (all.isPending) return <Skeleton className="h-40 w-full" />
      return (
        <WorktreesStep
          options={worktreeOptions}
          checked={chosen}
          onCheck={(worktrees, on) => dispatch({ type: 'check', worktrees, on })}
        />
      )
    }

    if (form.isError) return <RunFailure message={form.error.message} />
    if (!form.data) return <Skeleton className="h-40 w-full" />
    if (step === 'profile')
      return (
        <ProfileStep
          worktrees={chosen}
          options={profiles}
          value={picked}
          onChange={(value) => dispatch({ type: 'pick', profile: value })}
        />
      )

    if (unplanned)
      return <Refusal refusal={refusal} message={unplanned} onConsent={(to) => dispatch({ type: 'consent', to })} />
    if (starting.length > 0 && !plan) return <Skeleton className="h-40 w-full" />
    return (
      <ConfirmStep
        plan={plan}
        warnings={planned.data?.warnings ?? []}
        options={worktreeOptions.filter(({ value }) => starting.includes(value))}
        holds={worktreeOptions.flatMap((option) => option.holds ?? [])}
        others={others}
        left={left}
      />
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Start a profile</DialogTitle>
        <DialogDescription>Runs a profile's jobs in the worktrees you pick.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper steps={shownSteps.map((one) => ({ title: stepTitle[one], state: state(one) }))} />
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
            {startLabel(plan?.profile || picked, starting)}
          </ReasonButton>
        )}
      </DialogFooter>
    </>
  )
}

function startLabel(profile: string, worktrees: string[]) {
  const what = profile ? `Start ${profile}` : 'Start its jobs'
  return worktrees.length > 1 ? `${what} ${where(worktrees)}` : what
}
