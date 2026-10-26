import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useReducer } from 'react'
import { dryRunQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Details } from '@/features/worktrees/components/details'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { type Fact, Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { ChildrenStep } from '@/features/worktrees/remove/components/children-step'
import { RemovalItem } from '@/features/worktrees/remove/components/removal-item'
import { listed } from '@/features/worktrees/remove/utils/listed'
import { requestOf, unitsOf, unsafe } from '@/features/worktrees/remove/utils/plan'
import { initialWizard, type Step, wizardReducer } from '@/features/worktrees/remove/utils/wizard-state'
import { sharedWarnings } from '@/features/worktrees/remove/utils/words'

export function RemoveWizard({
  repo,
  worktrees,
  onStart,
  onCancel,
}: {
  repo: string
  worktrees: string[]
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const [{ step: chosen, choices, reparent }, dispatch] = useReducer(wizardReducer, initialWizard)

  // forced, the review plans every worktree asked, the unsafe ones with what keeps them: the choices are the person's
  const review = useQuery(
    dryRunQuery(repo, 'clean', [
      { path: 'worktree', values: worktrees },
      { path: 'force', values: ['true'] },
    ]),
  )
  const removals = review.data?.plan?.worktrees ?? []
  const undecided = removals.filter((removal) => unsafe(removal) && !choices[removal.worktree])
  const answers = requestOf({ removals, choices, reparent })
  const going = answers[0]?.values ?? []
  // a choice plans again: the plan shown stays until the next one lands
  const planned = useQuery({
    ...dryRunQuery(repo, 'clean', answers),
    enabled: undecided.length === 0 && going.length > 0,
    placeholderData: keepPreviousData,
  })
  const plan = planned.isError || planned.data?.error ? undefined : planned.data?.plan
  const moves = plan?.moves ?? []
  const fingerprint = planned.data?.fingerprint
  // the buttons never act on the plan a choice is replacing
  const fresh = plan !== undefined && !planned.isPlaceholderData
  const unplanned = planned.isError ? planned.error.message : planned.data?.error?.message

  // the step is the one reached, unless the facts moved under it: a worktree read again unsafe has its choice to make,
  // and children all gone leave nothing to ask
  const stepOf = (): Step => {
    if (undecided.length > 0) return 'worktrees'
    if (chosen === 'children' && fresh && moves.length === 0) return 'confirm'
    return chosen
  }
  const step = stepOf()
  const safe = removals.filter((removal) => !unsafe(removal))

  const state = (of: Step): StepState => {
    const order: Step[] = ['worktrees', 'children', 'confirm']
    if (of === step) return 'now'
    if (of === 'children' && moves.length === 0 && step === 'confirm') return 'skipped'
    return order.indexOf(of) < order.indexOf(step) ? 'done' : 'todo'
  }

  const next = () => {
    if (step === 'worktrees') return dispatch({ type: 'go', to: moves.length > 0 ? 'children' : 'confirm' })
    dispatch({ type: 'go', to: 'confirm' })
  }

  const back = () => dispatch({ type: 'go', to: step === 'confirm' && moves.length > 0 ? 'children' : 'worktrees' })

  // a Next that can't be pressed says why, so the step needs no sentence of its own
  const blocked = () => {
    if (undecided.length > 0) return `Choose for ${listed(undecided.map((removal) => removal.worktree))} first`
    if (review.data && going.length === 0) return 'Every worktree is kept: nothing to remove'
    return undefined
  }

  const counted = review.data ? going : worktrees
  const title = () => {
    if (counted.length === 0) return 'Remove worktrees'
    return counted.length === 1 ? `Remove ${counted[0]}` : `Remove ${counted.length} worktrees`
  }

  const removing = plan?.worktrees ?? []
  const warnings = (planned.data?.warnings ?? []).filter(
    (warning) => warning.code !== sharedWarnings.jobsLeft && warning.code !== sharedWarnings.dataKept,
  )
  const hooks = removing.flatMap((removal) => removal.hooks ?? []).length
  const keeping = removing.filter((removal) => (removal.namespaces ?? []).length > 0).length
  const summary = [
    hooks > 0 && `${hooks} on_clean hook${hooks === 1 ? '' : 's'} run first`,
    keeping > 0 && `data stays for ${keeping}`,
  ]
    .filter(Boolean)
    .join(', ')

  const factsOf = (removal: (typeof removing)[number]): Fact[] => [
    ...(removal.path ? [{ label: 'Folder', values: [removal.path], mono: true }] : []),
    ...((removal.hooks ?? []).length > 0 ? [{ label: 'on_clean', values: removal.hooks ?? [], mono: true }] : []),
    ...((removal.namespaces ?? []).length > 0
      ? [
          {
            label: 'Data',
            values: [`${removal.namespaces?.join(', ')} stays in its services`],
            line: 'wtm-next does not drop it yet',
          },
        ]
      : []),
  ]

  const body = () => {
    if (review.isPending) return <Skeleton className="h-40 w-full" />
    if (review.isError) return <p className="text-sm text-destructive">{review.error.message}</p>
    if (review.data.error) return <p className="text-sm text-destructive">{review.data.error.message}</p>

    if (step === 'worktrees')
      return (
        <ItemList label="Worktrees">
          {[...removals.filter(unsafe), ...safe].map((removal) => (
            <RemovalItem
              key={removal.worktree}
              removal={removal}
              choice={choices[removal.worktree]}
              onChoice={(choice) => dispatch({ type: 'choose', worktree: removal.worktree, choice })}
            />
          ))}
        </ItemList>
      )

    if (step === 'children')
      return (
        <ChildrenStep
          moves={moves}
          reparent={reparent}
          onReparent={(to) => dispatch({ type: 'reparent', reparent: to })}
        />
      )

    if (!plan) return !unplanned && <Skeleton className="h-40 w-full" />
    return (
      <>
        <ItemList label="Worktrees">
          {removing.map((removal) => (
            <RemovalItem
              key={removal.worktree}
              removal={removal}
              moves={moves.filter((move) => move.from === removal.branch)}
              reparent={reparent}
            />
          ))}
        </ItemList>
        <PlanWarnings warnings={warnings} />
        <Details label="Details" summary={summary}>
          {removing.map((removal) => (
            <Facts key={removal.worktree} heading={removal.worktree} facts={factsOf(removal)} />
          ))}
        </Details>
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title()}</DialogTitle>
        <DialogDescription>Each goes with its local branch and its record; the remote branch stays.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Worktrees', state: state('worktrees') },
            { title: 'Children', state: state('children') },
            { title: 'Confirm', state: state('confirm') },
          ]}
        />
      </DialogBand>

      <DialogScroll>
        {unplanned && <p className="text-sm text-destructive">{unplanned}</p>}
        {body()}
      </DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {step !== 'worktrees' && (
          <Button variant="outline" onClick={back}>
            Back
          </Button>
        )}
        {step !== 'confirm' && <NextButton disabled={!fresh} why={blocked()} onClick={next} />}
        {step === 'confirm' && (
          <Button
            variant="destructive"
            disabled={!fresh || !fingerprint}
            onClick={() => plan && fingerprint && onStart({ answers, fingerprint, units: unitsOf(plan) })}
          >
            {going.length === 1 ? `Remove ${going[0]}` : `Remove ${going.length} worktrees`}
          </Button>
        )}
      </DialogFooter>
    </>
  )
}

// aria-disabled, not disabled: a disabled button takes no focus nor pointer, so its reason would never show
function NextButton({ disabled, why, onClick }: { disabled: boolean; why: string | undefined; onClick: () => void }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          aria-disabled={disabled}
          className="aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
          onClick={() => {
            if (!disabled) onClick()
          }}
        >
          Next
        </Button>
      </TooltipTrigger>
      {disabled && why && <TooltipContent>{why}</TooltipContent>}
    </Tooltip>
  )
}
