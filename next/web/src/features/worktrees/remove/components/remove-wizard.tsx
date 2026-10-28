import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useReducer, useState } from 'react'
import { dryRunQuery } from '@/api/queries/commands'
import type { DomainWarning, TeardownRemoval } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { ChildrenStep } from '@/features/worktrees/components/children-step'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { NextButton } from '@/features/worktrees/components/next-button'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { RemovalDetails } from '@/features/worktrees/components/removal-details'
import { RemovalItem } from '@/features/worktrees/components/removal-item'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { requestOf } from '@/features/worktrees/remove/utils/plan'
import { searched } from '@/features/worktrees/utils/filters'
import { planWide } from '@/features/worktrees/utils/outcome'
import { plain, riskOf, unitsOf, unsafe } from '@/features/worktrees/utils/removal'
import { initialWizard, type Step, wizardReducer } from '@/features/worktrees/utils/wizard-state'

export function RemoveWizard({
  repo,
  worktrees,
  left,
  prechecked,
  notice,
  onStart,
  onCancel,
}: {
  repo: string
  worktrees: string[]
  left: Left[]
  prechecked: boolean
  notice?: DomainWarning | undefined
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const [{ step: chosen, checked, confirmed, reparent }, dispatch] = useReducer(wizardReducer, initialWizard)
  const [query, setQuery] = useState('')

  // forced, the review plans every worktree asked, the unsafe ones with what keeps them: the choices are the person's
  const review = useQuery(
    dryRunQuery(repo, 'clean', [
      { path: 'worktree', values: worktrees },
      { path: 'force', values: ['true'] },
    ]),
  )
  const removals = review.data?.plan?.worktrees ?? []
  // an unsafe worktree starts unchecked: none is touched unless asked for. A tick holds for the risk it was given
  // under, an untick whatever the risk: a worktree the person left out never comes back on its own
  const ticked = (removal: TeardownRemoval) => {
    const tick = checked[removal.worktree]
    return tick && (!tick.on || tick.risk === riskOf(removal)) ? tick : undefined
  }
  const isChecked = (removal: TeardownRemoval) => ticked(removal)?.on ?? (prechecked && !unsafe(removal))
  const check = (rows: TeardownRemoval[], on: boolean) =>
    dispatch({
      type: 'check',
      rows: rows.map((removal) => ({ worktree: removal.worktree, risk: riskOf(removal) })),
      on,
    })
  const going = removals.filter(isChecked)
  const answers = requestOf({ going, reparent })
  const planned = useQuery({
    ...dryRunQuery(repo, 'clean', answers),
    enabled: going.length > 0,
    placeholderData: keepPreviousData,
  })
  const plan = planned.isError || planned.data?.error ? undefined : planned.data?.plan
  const moves = plan?.moves ?? []
  const fingerprint = planned.data?.fingerprint
  // the buttons never act on the plan a choice is replacing; nor force a worktree the plan finds unsafe unless the
  // person checked it under that very risk: the two reads land in any order
  const consented = (plan?.worktrees ?? []).every((removal) => !unsafe(removal) || ticked(removal)?.on)
  const fresh = plan !== undefined && !planned.isPlaceholderData && !planned.isFetching && consented
  const unplanned = planned.isError ? planned.error.message : planned.data?.error?.message

  // the step is the one reached, unless the facts moved under it: once the worktrees were confirmed, one that left the
  // run or joined it since (its risk changed) sends the wizard back to say it; children all gone leave nothing to ask
  const goingNames = going.map((removal) => removal.worktree)
  const turned =
    confirmed.length > 0 &&
    (goingNames.some((worktree) => !confirmed.includes(worktree)) ||
      removals.some((removal) => confirmed.includes(removal.worktree) && !goingNames.includes(removal.worktree)))
  // back to the worktrees for good, not only on screen: only Next leaves them again, never a tick nor a read
  if (turned && chosen !== 'worktrees') dispatch({ type: 'go', to: 'worktrees' })
  const stepOf = (): Step => {
    if (turned) return 'worktrees'
    if (chosen === 'children' && fresh && moves.length === 0) return 'confirm'
    return chosen
  }
  const step = stepOf()
  const visible = removals.filter((removal) => searched(removal.worktree, query))

  const state = (of: Step): StepState => {
    const order: Step[] = ['worktrees', 'children', 'confirm']
    if (of === step) return 'now'
    if (of === 'children' && moves.length === 0 && step === 'confirm') return 'skipped'
    return order.indexOf(of) < order.indexOf(step) ? 'done' : 'todo'
  }

  const next = () => {
    if (step === 'worktrees')
      return dispatch({
        type: 'go',
        to: moves.length > 0 ? 'children' : 'confirm',
        confirmed: going.map((removal) => removal.worktree),
      })
    dispatch({ type: 'go', to: 'confirm' })
  }

  const back = () => dispatch({ type: 'go', to: step === 'confirm' && moves.length > 0 ? 'children' : 'worktrees' })

  // a Next that can't be pressed says why, so the step needs no sentence of its own
  const blocked = () => (review.data && going.length === 0 ? 'Check a worktree to remove it' : undefined)

  const leftRows = left.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)

  const removing = plan?.worktrees ?? []
  const warnings = step === 'confirm' ? (planned.data?.warnings ?? []).filter(planWide) : []
  const body = () => {
    if (review.isPending) return <Skeleton className="h-40 w-full" />
    if (review.isError) return <p className="text-sm text-destructive">{review.error.message}</p>
    if (review.data.error) return <p className="text-sm text-destructive">{review.data.error.message}</p>

    if (step === 'worktrees')
      return (
        <ItemList
          label="Worktrees"
          header={
            <ListHeader
              total={removals.length}
              visible={visible.map((removal) => removal.worktree)}
              bulk={visible.filter((removal) => !unsafe(removal)).map((removal) => removal.worktree)}
              checked={going.map((removal) => removal.worktree)}
              query={query}
              onQuery={setQuery}
              onCheck={(names, on) =>
                check(
                  visible.filter((removal) => names.includes(removal.worktree)),
                  on,
                )
              }
            />
          }
          left={leftRows}
        >
          {visible.map((removal) => (
            <RemovalItem
              key={removal.worktree}
              removal={removal}
              checked={isChecked(removal)}
              onCheck={(on) => check([removal], on)}
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
        <ItemList label="Worktrees" left={leftRows}>
          {removing.map((removal) => (
            <RemovalItem
              key={removal.worktree}
              removal={removal}
              moves={moves.filter((move) => move.from === removal.branch)}
              reparent={reparent}
            />
          ))}
        </ItemList>
        <RemovalDetails removals={removing} />
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Remove worktrees</DialogTitle>
        <DialogDescription>
          {plain(removals)
            ? 'Each goes with its local branch and its record; the remote branch stays.'
            : 'Each goes with its record; its row says what becomes of its branch.'}
        </DialogDescription>
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
        <PlanWarnings warnings={notice ? [notice, ...warnings] : warnings} />
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
            {going.length === 1 ? 'Remove' : `Remove ${going.length} worktrees`}
          </Button>
        )}
      </DialogFooter>
    </>
  )
}
