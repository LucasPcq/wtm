import { useQuery } from '@tanstack/react-query'
import { useReducer } from 'react'
import { dryRunQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainAnswered, DomainError, DomainField } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { ItemList } from '@/features/worktrees/components/item-list'
import { LeftItem } from '@/features/worktrees/components/left-item'
import { NextButton } from '@/features/worktrees/components/next-button'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import type { RunUpAsk, RunUpRan } from '@/features/worktrees/hooks/use-run-up-dialog'
import { ConfirmStep } from '@/features/worktrees/run-up/components/confirm-step'
import { ProfileStep } from '@/features/worktrees/run-up/components/profile-step'
import { WorktreesStep } from '@/features/worktrees/run-up/components/worktrees-step'
import { unitsOf } from '@/features/worktrees/run-up/utils/plan'
import { steps, wizardReducer } from '@/features/worktrees/run-up/utils/wizard-state'
import { where } from '@/features/worktrees/run-up/utils/words'

const optionsOf = (fields: DomainField[] | undefined, path: string) => {
  const field = fields?.find((one) => one.path === path)
  return field?.kind === 'multi_select' || field?.kind === 'select' ? field : undefined
}

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
  const [{ step, checked, profile }, dispatch] = useReducer(wizardReducer, {
    step: ask.step,
    checked: ask.worktrees,
    profile: ask.profile,
  })

  const all = useQuery(questionsQuery(repo, 'run-up'))
  const worktreeField = optionsOf(all.data?.fields, 'worktree')
  const worktreeOptions = worktreeField?.options ?? []
  const names = worktreeOptions.map((option) => option.value)
  // in the core's order, whatever order they were checked in
  const chosen = all.data ? names.filter((name) => checked.includes(name)) : checked

  // the profiles are asked for the worktrees chosen: each says which of them run it already
  const form = useQuery({
    ...questionsQuery(repo, 'run-up', [{ path: 'worktree', values: chosen }]),
    enabled: chosen.length > 0,
  })
  const profileField = optionsOf(form.data?.fields, 'profile')
  const profiles = profileField?.options ?? []
  const picked = profile || (profileField?.kind === 'select' ? profileField.value : '')
  // opened on Confirm with no profile the core can pick (no default among several): the profile is asked first
  if (step === 'confirm' && form.isSuccess && picked === '') dispatch({ type: 'go', to: 'profile' })

  const answers: DomainAnswered[] = [
    { path: 'worktree', values: chosen },
    { path: 'profile', values: [picked] },
  ]
  const planned = useQuery({
    ...dryRunQuery(repo, 'run-up', answers),
    enabled: step === 'confirm' && chosen.length > 0 && picked !== '',
  })
  const plan = planned.data?.error ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  const refusal: DomainError | undefined = planned.data?.error ?? undefined
  const unplanned = planned.isError ? planned.error.message : refusal?.message

  const state = (of: (typeof steps)[number]): StepState => {
    if (of === step) return 'now'
    return steps.indexOf(of) < steps.indexOf(step) ? 'done' : 'todo'
  }

  const blocked = (): string | undefined => {
    if (step === 'worktrees') return chosen.length === 0 ? 'Check at least one worktree' : undefined
    return picked === '' ? 'Pick a profile' : undefined
  }

  const runBlocked = (): string | undefined => {
    if (unplanned) return 'The plan was refused'
    if (!plan || !fingerprint || planned.isFetching) return 'Planning…'
    return undefined
  }

  const next = () => dispatch({ type: 'go', to: step === 'worktrees' ? 'profile' : 'confirm' })
  const back = () => dispatch({ type: 'go', to: step === 'confirm' ? 'profile' : 'worktrees' })

  const run = () => {
    if (!plan || !fingerprint) return
    onStart({ answers, fingerprint, units: unitsOf(plan) }, { worktrees: chosen, profile: plan.profile, plan })
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
    if (step === 'profile') {
      if (!form.data) return <Skeleton className="h-40 w-full" />
      return (
        <ProfileStep
          worktrees={chosen}
          options={profiles}
          value={picked}
          onChange={(value) => dispatch({ type: 'pick', profile: value })}
        />
      )
    }

    if (unplanned)
      return (
        <Alert
          tone="red"
          description={unplanned}
          below={refusal?.follow_up && <CommandLine followUp={refusal.follow_up} />}
        />
      )
    if (!plan) return <Skeleton className="h-40 w-full" />
    return (
      <>
        <ConfirmStep plan={plan} warnings={planned.data?.warnings ?? []} />
        <ItemList left={ask.left.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)}>
          {[]}
        </ItemList>
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Start a profile</DialogTitle>
        <DialogDescription>Runs a profile's jobs in the worktrees you pick.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Worktrees', state: state('worktrees') },
            { title: 'Profile', state: state('profile') },
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
            {chosen.length === 1 ? `Start ${picked}` : `Start ${picked} ${where(chosen)}`}
          </ReasonButton>
        )}
      </DialogFooter>
    </>
  )
}
