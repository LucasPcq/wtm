import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useReducer, useState } from 'react'
import { dryRunQuery, reviewQuery } from '@/api/queries/commands'
import type { PruneCandidate } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { ChildrenStep } from '@/features/worktrees/components/children-step'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { ItemList } from '@/features/worktrees/components/item-list'
import { LeftItem } from '@/features/worktrees/components/left-item'
import { ListHeader } from '@/features/worktrees/components/list-header'
import { NextButton } from '@/features/worktrees/components/next-button'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { RemovalDetails } from '@/features/worktrees/components/removal-details'
import { RemovalItem } from '@/features/worktrees/components/removal-item'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Left } from '@/features/worktrees/hooks/use-remove-dialog'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { NothingToPrune } from '@/features/worktrees/prune/components/nothing-to-prune'
import { ReasonLine } from '@/features/worktrees/prune/components/reason-line'
import { kept, requestOf } from '@/features/worktrees/prune/utils/plan'
import { notFinished } from '@/features/worktrees/prune/utils/words'
import { blockerWords } from '@/features/worktrees/utils/blocker-words'
import { searched } from '@/features/worktrees/utils/filters'
import { planWide } from '@/features/worktrees/utils/outcome'
import { riskOf, unitsOf, unsafe } from '@/features/worktrees/utils/removal'
import { sentence } from '@/features/worktrees/utils/sentence'
import { initialWizard, type Step, wizardReducer } from '@/features/worktrees/utils/wizard-state'

export function PruneWizard({
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
  const [{ step: chosen, checked, confirmed, reparent }, dispatch] = useReducer(wizardReducer, initialWizard)
  const [query, setQuery] = useState('')
  const [dropped, setDropped] = useState<Left[]>([])

  // forced, the review lists every candidate with what makes it unsafe, and is the one read that fetches origin
  const review = useQuery(
    reviewQuery(repo, 'prune', [
      ...(worktrees.length > 0 ? [{ path: 'worktree', values: worktrees }] : []),
      { path: 'force', values: ['true'] },
    ]),
  )
  const candidates = review.data?.plan?.worktrees ?? []
  // an unsafe worktree starts unchecked: none is touched unless asked for. A tick holds for the risk it was given
  // under, an untick whatever the risk: a worktree the person left out never comes back on its own
  const ticked = (candidate: PruneCandidate) => {
    const tick = checked[candidate.worktree]
    return tick && (!tick.on || tick.risk === riskOf(candidate)) ? tick : undefined
  }
  const isChecked = (candidate: PruneCandidate) => ticked(candidate)?.on ?? !unsafe(candidate)
  const check = (rows: PruneCandidate[], on: boolean) =>
    dispatch({
      type: 'check',
      rows: rows.map((candidate) => ({ worktree: candidate.worktree, risk: riskOf(candidate) })),
      on,
    })
  const going = candidates.filter(isChecked)
  const goingNames = going.map((candidate) => candidate.worktree)
  const answers = requestOf({ named: worktrees, candidates, going, reparent })
  const planned = useQuery({
    ...dryRunQuery(repo, 'prune', answers),
    enabled: going.length > 0,
    placeholderData: keepPreviousData,
  })

  // a worktree named that is no longer done with since the review leaves the run, and says why under Left as they are
  const noLonger = (planned.data?.error?.fields ?? []).flatMap(({ code, params }) =>
    code === notFinished && params?.value && goingNames.includes(params.value)
      ? [{ worktree: params.value, why: `No longer done with: ${params.why ?? ''}` }]
      : [],
  )
  if (noLonger.length > 0) {
    check(
      candidates.filter((candidate) => noLonger.some((one) => one.worktree === candidate.worktree)),
      false,
    )
    setDropped([...dropped.filter((one) => !noLonger.some((now) => now.worktree === one.worktree)), ...noLonger])
  }
  const left = dropped.filter((one) => !goingNames.includes(one.worktree))

  const plan = planned.isError || planned.data?.error ? undefined : planned.data?.plan
  const pruning = (plan?.worktrees ?? []).filter((candidate) => !kept(candidate))
  const keeping = (plan?.worktrees ?? []).filter(kept)
  const moves = plan?.moves ?? []
  const fingerprint = planned.data?.fingerprint
  // the buttons never act on the plan a choice is replacing; nor force a worktree the plan finds unsafe unless the
  // person checked it under that very risk: the two reads land in any order
  const consented = pruning.every((candidate) => !unsafe(candidate) || ticked(candidate)?.on)
  const fresh = plan !== undefined && !planned.isPlaceholderData && !planned.isFetching && consented
  const unplanned = noLonger.length > 0 ? undefined : (planned.error?.message ?? planned.data?.error?.message)

  // the step is the one reached, unless the facts moved under it: once the worktrees were confirmed, one that left the
  // run or joined it since (its risk changed) sends the wizard back to say it, unless the plan said why it left
  const turned =
    confirmed.length > 0 &&
    (goingNames.some((worktree) => !confirmed.includes(worktree)) ||
      candidates.some(
        ({ worktree }) =>
          confirmed.includes(worktree) &&
          !goingNames.includes(worktree) &&
          !left.some((one) => one.worktree === worktree),
      ))
  // back to the worktrees for good, not only on screen: only Next leaves them again, never a tick nor a read
  if (turned && chosen !== 'worktrees') dispatch({ type: 'go', to: 'worktrees' })
  const stepOf = (): Step => {
    if (turned) return 'worktrees'
    if (chosen === 'children' && fresh && moves.length === 0) return 'confirm'
    return chosen
  }
  const step = stepOf()
  const visible = candidates.filter((candidate) => searched(candidate.worktree, query))
  // shortcut: an empty review is read here as nothing to prune; once the core ends an empty plan itself (#245), read that
  const nothing = review.data !== undefined && !review.data.error && candidates.length === 0

  const state = (of: Step): StepState => {
    const order: Step[] = ['worktrees', 'children', 'confirm']
    if (of === step) return 'now'
    if (of === 'children' && moves.length === 0 && step === 'confirm') return 'skipped'
    return order.indexOf(of) < order.indexOf(step) ? 'done' : 'todo'
  }

  const next = () => {
    if (step === 'worktrees')
      return dispatch({ type: 'go', to: moves.length > 0 ? 'children' : 'confirm', confirmed: goingNames })
    dispatch({ type: 'go', to: 'confirm' })
  }

  const back = () => dispatch({ type: 'go', to: step === 'confirm' && moves.length > 0 ? 'children' : 'worktrees' })

  // a Next that can't be pressed says why, so the step needs no sentence of its own
  const blocked = () => {
    if (review.isPending) return 'Reading GitHub and origin'
    return going.length === 0 ? 'Check a worktree to prune it' : undefined
  }

  const leftRows = (rows: Left[]) =>
    rows.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)
  const keptRows = keeping.map((candidate) => ({
    worktree: candidate.worktree,
    why: `Kept: ${(candidate.blockers ?? []).map((blocker) => blockerWords[blocker](candidate)).join(', ')}`,
  }))
  // only the review fetched origin: its plan-wide warnings hold for every plan after it, the latest saying each one
  const alerts = [...(review.data?.warnings ?? []), ...(step === 'confirm' ? (planned.data?.warnings ?? []) : [])]
  const warnings = alerts
    .filter(planWide)
    .filter((one, i, all) => !all.slice(i + 1).some((later) => later.code === one.code))

  const body = () => {
    if (review.isPending) return <Skeleton className="h-40 w-full" />
    if (review.isError) return <p className="text-sm text-destructive">{review.error.message}</p>
    if (review.data.error) return <p className="text-sm text-destructive">{sentence(review.data.error.message)}</p>

    if (nothing) return <NothingToPrune />

    if (step === 'worktrees')
      return (
        <ItemList
          label="Worktrees"
          header={
            <ListHeader
              total={candidates.length}
              visible={visible.map((candidate) => candidate.worktree)}
              bulk={visible.filter((candidate) => !unsafe(candidate)).map((candidate) => candidate.worktree)}
              checked={goingNames}
              query={query}
              onQuery={setQuery}
              onCheck={(names, on) =>
                check(
                  visible.filter((candidate) => names.includes(candidate.worktree)),
                  on,
                )
              }
            />
          }
          left={leftRows(left)}
        >
          {visible.map((candidate) => (
            <RemovalItem
              key={candidate.worktree}
              removal={candidate}
              verb="Prune"
              reason={<ReasonLine candidate={candidate} />}
              checked={isChecked(candidate)}
              onCheck={(on) => check([candidate], on)}
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
        <ItemList label="Worktrees" left={leftRows([...keptRows, ...left])}>
          {pruning.map((candidate) => (
            <RemovalItem
              key={candidate.worktree}
              removal={candidate}
              reason={<ReasonLine candidate={candidate} />}
              moves={moves.filter((move) => move.from === candidate.branch)}
              reparent={reparent}
            />
          ))}
        </ItemList>
        <RemovalDetails removals={pruning} />
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Prune worktrees</DialogTitle>
        <DialogDescription>Each goes with its local branch and its record; the remote branch stays.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        {!nothing && (
          <Stepper
            steps={[
              { title: 'Worktrees', state: state('worktrees') },
              { title: 'Children', state: state('children') },
              { title: 'Confirm', state: state('confirm') },
            ]}
          />
        )}
      </DialogBand>

      <DialogScroll>
        {unplanned && <p className="text-sm text-destructive">{sentence(unplanned)}</p>}
        {step !== 'children' && <PlanWarnings warnings={warnings} />}
        {body()}
      </DialogScroll>

      <DialogFooter className="items-center">
        {nothing ? (
          <Button onClick={onCancel}>Done</Button>
        ) : (
          <>
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
                {pruning.length === 1 ? 'Prune' : `Prune ${pruning.length} worktrees`}
              </Button>
            )}
          </>
        )}
      </DialogFooter>
    </>
  )
}
