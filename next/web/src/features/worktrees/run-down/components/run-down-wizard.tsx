import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { dryRunQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainAnswered, DomainOption } from '@/api/types.gen'
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
import { RunWorktreesStep } from '@/features/worktrees/components/run-worktrees-step'
import { Stepper } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import type { RunDownAsk, RunDownRan, RunDownStep } from '@/features/worktrees/hooks/use-run-down-dialog'
import { ConfirmStep } from '@/features/worktrees/run-down/components/confirm-step'
import { unitsOf } from '@/features/worktrees/run-down/utils/plan'
import { stopLabel } from '@/features/worktrees/run-down/utils/words'
import { where } from '@/features/worktrees/utils/run-words'

const runsSomething = ({ up = [], crashed = 0, holds = [] }: DomainOption) =>
  up.length > 0 || crashed > 0 || holds.length > 0

export function RunDownWizard({
  repo,
  ask,
  onStop,
  onCancel,
}: {
  repo: string
  ask: RunDownAsk
  onStop: (start: Start, ran: RunDownRan) => void
  onCancel: () => void
}) {
  const [step, setStep] = useState<RunDownStep>(ask.picking ? 'worktrees' : 'confirm')
  const [checked, setChecked] = useState(ask.worktrees)

  const all = useQuery(questionsQuery(repo, 'run-down'))
  const field = all.data?.fields.find((one) => one.path === 'worktree')
  const options = field?.kind === 'multi_select' ? field.options : []
  // in the core's order, whatever order they were checked in
  const chosen = all.data ? options.map(({ value }) => value).filter((name) => checked.includes(name)) : checked

  const answers: DomainAnswered[] = [{ path: 'worktree', values: chosen }]
  const planned = useQuery({
    ...dryRunQuery(repo, 'run-down', answers),
    enabled: step === 'confirm' && chosen.length > 0,
  })
  const plan = planned.data?.error ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  const refusal = planned.data?.error ?? undefined
  const unplanned = planned.isError ? planned.error.message : refusal?.message
  const stopping = (plan?.worktrees ?? []).filter((target) => (target.jobs ?? []).length > 0).map((t) => t.worktree)

  // a worktree asked for that the core no longer offers (removed meanwhile) has nothing to stop
  const gone = step === 'confirm' && all.data !== undefined && chosen.length === 0

  const runBlocked = (): string | undefined => {
    if (gone) return `Nothing runs ${where(checked)}`
    if (unplanned) return 'The plan was refused'
    if (!plan || !fingerprint || planned.isFetching) return 'Planning…'
    if (stopping.length === 0) return `Nothing runs ${where(chosen)}`
    return undefined
  }

  const check = (worktrees: string[], on: boolean) =>
    setChecked((now) =>
      on ? [...now, ...worktrees.filter((one) => !now.includes(one))] : now.filter((one) => !worktrees.includes(one)),
    )

  const run = () => {
    if (!plan || !fingerprint) return
    onStop({ answers, fingerprint, units: unitsOf(plan) }, { worktrees: stopping, plan })
  }

  const body = () => {
    if (all.isError) return <RunFailure message={all.error.message} />
    if (!all.data) return <Skeleton className="h-40 w-full" />
    if (step === 'worktrees')
      return (
        <RunWorktreesStep
          options={options.filter(runsSomething)}
          checked={chosen}
          verb="Stop in"
          note="only the worktrees that run something"
          onCheck={check}
        />
      )

    if (gone)
      return (
        <ItemList
          left={[...checked.map((worktree) => ({ worktree, why: 'nothing running' })), ...ask.left].map(
            ({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />,
          )}
        >
          {[]}
        </ItemList>
      )
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
      <ConfirmStep
        plan={plan}
        warnings={planned.data?.warnings ?? []}
        options={options.filter(({ value }) => stopping.includes(value))}
        left={ask.left}
      />
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Stop jobs</DialogTitle>
        <DialogDescription>Stops every job of the worktrees you pick (run down).</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Worktrees', state: step === 'worktrees' ? 'now' : 'done' },
            { title: 'Confirm', state: step === 'confirm' ? 'now' : 'todo' },
          ]}
        />
      </DialogBand>

      <DialogScroll>{body()}</DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {step === 'confirm' && ask.picking && (
          <Button variant="outline" onClick={() => setStep('worktrees')}>
            Back
          </Button>
        )}
        {step === 'worktrees' && (
          <NextButton
            disabled={chosen.length === 0}
            why={chosen.length === 0 ? 'Check at least one worktree' : undefined}
            onClick={() => setStep('confirm')}
          />
        )}
        {step === 'confirm' && (
          <ReasonButton disabled={runBlocked() !== undefined} why={runBlocked()} onClick={run}>
            {stopLabel(stopping)}
          </ReasonButton>
        )}
      </DialogFooter>
    </>
  )
}
