import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useReducer, useState } from 'react'
import { dryRunQuery, planQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainAnswered, DomainFollowUp, DomainWarning, SyncRebase } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import type { Left } from '@/features/worktrees/components/left-item'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { Stepper } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { StackStep } from '@/features/worktrees/sync/components/stack-step'
import { SyncConfirm } from '@/features/worktrees/sync/components/sync-confirm'
import {
  moves,
  noFetch,
  pushOnly,
  rebasesOf,
  requestOf,
  riskOf,
  unitsOf,
  unsafe,
  valuesOf,
} from '@/features/worktrees/sync/utils/plan'
import { stackOf } from '@/features/worktrees/sync/utils/stack'
import { initialWizard, type Step, wizardReducer } from '@/features/worktrees/sync/utils/wizard-state'
import { names } from '@/features/worktrees/sync/utils/words'
import { refusalOf } from '@/features/worktrees/utils/sentence'

export function SyncWizard({
  repo,
  answers: asked,
  left,
  picking,
  deciding,
  notice,
  onStart,
  onReparent,
  onCancel,
}: {
  repo: string
  answers: DomainAnswered[]
  left: Left[]
  picking: boolean
  deciding: boolean
  notice?: DomainWarning | undefined
  onStart: (start: Start) => void
  onReparent: (followUp: DomainFollowUp) => void
  onCancel: () => void
}) {
  const opensOn: Step = picking || deciding ? 'worktrees' : 'confirm'
  const [{ step, checked, switches }, dispatch] = useReducer(
    wizardReducer,
    { answers: asked, step: opensOn },
    initialWizard,
  )
  const [query, setQuery] = useState('')
  const confirming = step === 'confirm'

  // the form never fetches: it gives the header's own pick and each worktree's uncommitted changes
  const form = useQuery(questionsQuery(repo, 'sync'))
  const field = form.data?.fields.find((one) => one.path === 'worktree')
  const options = field?.kind === 'multi_select' ? field.options : []
  const picked = field?.kind === 'multi_select' ? field.picked : []
  const named = picking ? picked : (valuesOf(asked, 'worktree') ?? [])
  const changesOf = (worktree: string) => options.find((option) => option.value === worktree)?.changes

  // the review is forced over what step 1 shows, so each dirty worktree shows its risk; the one fetch of the flow
  const review = useQuery(
    dryRunQuery(repo, 'sync', [
      picking ? { path: 'all', values: ['true'] } : { path: 'worktree', values: named },
      { path: 'force', values: ['true'] },
    ]),
  )
  const reviewed = review.data?.error ? undefined : review.data?.plan
  const rows = reviewed?.worktrees ?? []

  // a tick holds for the risk it was given under, an untick whatever the risk
  const ticked = (rebase: SyncRebase) => {
    const tick = checked[rebase.worktree]
    return tick && (!tick.on || tick.risk === riskOf(rebase)) ? tick : undefined
  }
  const isNamed = (rebase: SyncRebase) => ticked(rebase)?.on ?? (named.includes(rebase.worktree) && !unsafe(rebase))
  const going = rows.filter(isNamed)
  const untouched = going.length === named.length && going.every((rebase) => named.includes(rebase.worktree))
  const chosen = picking && untouched
  const answers = [
    ...requestOf({ chosen, named: going.map((rebase) => rebase.worktree), forced: going.some(unsafe), switches }),
    noFetch,
  ]
  const asking = reviewed !== undefined && (chosen || going.length > 0)
  const planned = useQuery({
    ...planQuery(repo, 'sync', { answers, readAt: review.dataUpdatedAt }),
    enabled: asking && !review.isFetching,
    placeholderData: keepPreviousData,
  })
  const plan = asking && !planned.isError && !planned.data?.error ? planned.data?.plan : undefined
  const fingerprint = planned.data?.fingerprint
  // forced for one dirty worktree checked, the plan forces every dirty one: none moves unless checked too
  const unconsented = (plan?.worktrees ?? []).filter((rebase) => unsafe(rebase) && moves(rebase) && !isNamed(rebase))
  const fresh =
    plan !== undefined &&
    !planned.isPlaceholderData &&
    !planned.isFetching &&
    !review.isFetching &&
    unconsented.length === 0
  const refusal = review.data?.error ?? (asking ? planned.data?.error : undefined)
  const empty = fresh && plan.empty === true
  const behind = (reviewed?.onto ?? []).filter(moves).map((onto) => onto.branch)
  const count = plan ? (pushOnly(plan) ? (plan.worktrees ?? []) : rebasesOf(plan)).filter(moves).length : 0

  const why = () => {
    if (review.isFetching) return 'Fetching origin'
    if (refusal?.follow_up?.command === 'reparent')
      return `Reparent ${names(valuesOf(refusal.follow_up.answers ?? [], 'worktree') ?? [])} to sync it`
    if (refusal) return refusalOf(refusal)
    if (planned.isFetching) return 'Planning'
    if (unconsented.length > 0)
      return `Check ${names(unconsented.map((rebase) => rebase.worktree))} too: forcing one forces every dirty worktree`
    if (going.length === 0 && !chosen) {
      const held = rows.filter((rebase) => named.includes(rebase.worktree) && unsafe(rebase))
      if (held.length > 0)
        return `Check ${names(held.map((rebase) => rebase.worktree))} to sync ${held.length === 1 ? 'it' : 'them'} anyway`
      return 'Check a worktree to sync it'
    }
    return undefined
  }

  const primary = () => {
    if (plan && pushOnly(plan)) return count > 1 ? `Push ${count} worktrees` : 'Push'
    return count > 1 ? `Sync ${count} worktrees` : 'Sync'
  }

  const refused = () =>
    refusal && (
      <Alert
        tone="red"
        title="Can't sync"
        description={refusalOf(refusal)}
        below={
          refusal.follow_up &&
          (refusal.follow_up.command === 'reparent' ? (
            <Button className="w-full" onClick={() => refusal.follow_up && onReparent(refusal.follow_up)}>
              Reparent onto {valuesOf(refusal.follow_up.answers ?? [], 'to')?.[0] ?? 'its base'}
            </Button>
          ) : (
            <CommandLine followUp={refusal.follow_up} />
          ))
        }
      />
    )

  const body = () => {
    if (review.isPending || (confirming && asking && !planned.data)) return <Skeleton className="h-40 w-full" />
    if (review.isError) return <Alert tone="red" description={review.error.message} />
    if (planned.isError) return <Alert tone="red" description={planned.error.message} />
    if (review.data.error) return refused()

    if (!confirming)
      return (
        <>
          {refused()}
          <StackStep
            stack={stackOf(reviewed ?? { onto: [], worktrees: [] })}
            plan={plan}
            picking={picking}
            isNamed={isNamed}
            onCheck={(list, on) =>
              dispatch({ type: 'check', rows: list.map((r) => ({ worktree: r.worktree, risk: riskOf(r) })), on })
            }
            changesOf={changesOf}
            switches={switches}
            onSwitch={(name, on) => {
              // off, the worktrees under the named stay where they are: none checked anyway goes on its own
              if (name === 'descendants' && !on)
                dispatch({
                  type: 'check',
                  rows: rows.filter((r) => r.descendant).map((r) => ({ worktree: r.worktree, risk: riskOf(r) })),
                  on: false,
                })
              dispatch({ type: 'switch', name, on })
            }}
            query={query}
            onQuery={setQuery}
          />
        </>
      )

    if (refusal) return refused()
    if (!plan) return <Skeleton className="h-40 w-full" />
    if (empty)
      return (
        <>
          <PlanWarnings warnings={planned.data?.warnings ?? []} />
          <Empty className="gap-4 p-6">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <RefreshCw />
              </EmptyMedia>
              <EmptyTitle>Nothing to sync</EmptyTitle>
              <EmptyDescription>
                A worktree needs a sync once its parent has commits it lacks, or its parent is behind origin.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        </>
      )
    return (
      <SyncConfirm
        plan={plan}
        warnings={planned.data?.warnings ?? []}
        left={left}
        review={rows}
        behind={behind}
        changesOf={changesOf}
        onReparent={onReparent}
      />
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Sync worktrees</DialogTitle>
        <DialogDescription>Each worktree is rebased onto its parent, parents first.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Worktrees', state: confirming ? 'done' : 'now' },
            { title: 'Confirm', state: confirming ? 'now' : 'todo' },
          ]}
        />
      </DialogBand>

      {/* each step opens at its top */}
      <DialogScroll key={step}>
        {notice && <PlanWarnings warnings={[notice]} />}
        {body()}
      </DialogScroll>

      <DialogFooter className="items-center">
        {confirming && empty ? (
          <Button onClick={onCancel}>Done</Button>
        ) : (
          <>
            <Button variant="ghost" className="mr-auto" onClick={onCancel}>
              Cancel
            </Button>
            {/* Sync only: every choice sits on step 1, so Back reaches it even from a row */}
            {confirming && (
              <Button variant="outline" onClick={() => dispatch({ type: 'step', step: 'worktrees' })}>
                Back
              </Button>
            )}
            {!confirming && (
              <ReasonButton
                disabled={!fresh}
                why={why() ?? (fresh ? undefined : 'Planning')}
                onClick={() => dispatch({ type: 'step', step: 'confirm' })}
              >
                Next
              </ReasonButton>
            )}
            {confirming && (
              <ReasonButton
                disabled={!fresh || !fingerprint}
                why={why()}
                onClick={() => plan && fingerprint && onStart({ answers, fingerprint, units: unitsOf(plan) })}
              >
                {primary()}
              </ReasonButton>
            )}
          </>
        )}
      </DialogFooter>
    </>
  )
}
