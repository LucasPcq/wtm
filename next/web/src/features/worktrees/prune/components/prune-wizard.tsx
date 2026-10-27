import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useReducer, useState } from 'react'
import { dryRunQuery, planQuery, reviewQuery } from '@/api/queries/commands'
import type { PruneCandidate } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { ChildrenStep } from '@/features/worktrees/components/children-step'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { NextButton } from '@/features/worktrees/components/next-button'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { RemovalDetails } from '@/features/worktrees/components/removal-details'
import { RemovalItem } from '@/features/worktrees/components/removal-item'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { CandidateList } from '@/features/worktrees/prune/components/candidate-list'
import { NothingToPrune } from '@/features/worktrees/prune/components/nothing-to-prune'
import { ReasonLine } from '@/features/worktrees/prune/components/reason-line'
import { leftOut } from '@/features/worktrees/prune/utils/left'
import { requestOf } from '@/features/worktrees/prune/utils/plan'
import { notFinished } from '@/features/worktrees/prune/utils/words'
import { blockerWords } from '@/features/worktrees/utils/blocker-words'
import { planWide } from '@/features/worktrees/utils/outcome'
import { kept, riskOf, unitsOf, unsafe } from '@/features/worktrees/utils/removal'
import { sentence } from '@/features/worktrees/utils/sentence'
import { initialWizard, type Step, wizardReducer } from '@/features/worktrees/utils/wizard-state'

export function PruneWizard({
  repo,
  worktrees,
  notice,
  onStart,
  onCancel,
}: {
  repo: string
  worktrees: string[]
  notice?: string | undefined
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const [{ step: chosen, checked, confirmed, reparent }, dispatch] = useReducer(wizardReducer, initialWizard)
  const [dropped, setDropped] = useState<Left[]>([])

  // forced, both list every candidate with what makes it unsafe; the review is the one read that fetches origin, and
  // says what fetching found; the rows read origin as it left it, again on every event, so a risk shows on its row.
  // The rows name no worktree: one opened on that stops being done with leaves them, rather than refusing them all
  const force = { path: 'force', values: ['true'] }
  const review = useQuery(
    reviewQuery(repo, 'prune', [...(worktrees.length > 0 ? [{ path: 'worktree', values: worktrees }] : []), force]),
  )
  const rows = useQuery({
    ...dryRunQuery(repo, 'prune', [force, { path: 'no_fetch', values: ['true'] }]),
    enabled: review.isSuccess,
  })
  const candidates = (rows.data?.plan?.worktrees ?? []).filter(
    (candidate) => worktrees.length === 0 || worktrees.includes(candidate.worktree),
  )
  // an unsafe worktree starts unchecked: none is touched unless asked for. A tick holds for the risk it was given
  // under, an untick whatever the risk: a worktree the person left out never comes back on its own
  const ticked = (candidate: PruneCandidate) => {
    const tick = checked[candidate.worktree]
    return tick && (!tick.on || tick.risk === riskOf(candidate)) ? tick : undefined
  }
  const isChecked = (candidate: PruneCandidate) => ticked(candidate)?.on ?? !unsafe(candidate)
  const check = (some: PruneCandidate[], on: boolean) =>
    dispatch({
      type: 'check',
      rows: some.map((candidate) => ({ worktree: candidate.worktree, risk: riskOf(candidate) })),
      on,
    })
  const going = candidates.filter(isChecked)
  const goingNames = going.map((candidate) => candidate.worktree)
  const answers = requestOf({ named: worktrees, candidates, going, reparent })
  // a plan is asked on the rows once read in this opening and settled: until then its key holds nothing, not the plan
  // an earlier opening cached under the same read
  const read = rows.isFetchedAfterMount && !rows.isFetching ? rows.dataUpdatedAt : 0
  const planned = useQuery({
    ...planQuery(repo, 'prune', { answers, readAt: read }),
    enabled: going.length > 0 && read > 0,
    placeholderData: keepPreviousData,
  })

  // a worktree named that is no longer done with since the review leaves the run, and says why under Left as they are;
  // only on this plan's own answer, never on one kept from an earlier opening that is being asked again
  const settled = !planned.isFetching && !planned.isPlaceholderData
  const noLonger = (settled ? (planned.data?.error?.fields ?? []) : []).flatMap(({ code, params }) =>
    code === notFinished && params?.value && goingNames.includes(params.value)
      ? [{ worktree: params.value, why: `No longer finished: ${params.why ?? ''}` }]
      : [],
  )
  if (noLonger.length > 0) {
    check(
      candidates.filter((candidate) => noLonger.some((one) => one.worktree === candidate.worktree)),
      false,
    )
    setDropped([...dropped.filter((one) => !noLonger.some((now) => now.worktree === one.worktree)), ...noLonger])
  }
  const left = leftOut({
    dropped,
    asked: [...new Set([...worktrees, ...confirmed, ...Object.keys(checked).filter((w) => checked[w]?.on)])],
    candidates: candidates.map((candidate) => candidate.worktree),
    going: goingNames,
  })

  const plan = planned.isError || planned.data?.error ? undefined : planned.data?.plan
  const pruning = (plan?.worktrees ?? []).filter((candidate) => !kept(candidate))
  const keeping = (plan?.worktrees ?? []).filter(kept)
  const moves = plan?.moves ?? []
  const fingerprint = planned.data?.fingerprint
  // the buttons never act on the plan a choice is replacing; nor force a worktree the plan finds unsafe unless the
  // person checked it under that very risk: the two reads land in any order
  const consented = pruning.every((candidate) => !unsafe(candidate) || ticked(candidate)?.on)
  const fresh = plan !== undefined && !planned.isPlaceholderData && !planned.isFetching && consented
  // a label never counts a plan refused or still asked: the checked rows stand in for it
  const count = fresh ? pruning.length : going.length
  const unplanned =
    !settled || noLonger.length > 0 ? undefined : (planned.error?.message ?? planned.data?.error?.message)

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
  // forced, the rows keep none: a plan that changes nothing is one with no candidate
  const nothing = rows.data?.plan?.empty === true

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
    if (review.isPending || rows.isPending) return 'Reading GitHub and origin'
    return going.length === 0 ? 'Check a worktree to prune it' : undefined
  }

  const leftRows = (some: Left[]) =>
    some.map(({ worktree, why }) => <LeftItem key={worktree} worktree={worktree} why={why} />)
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
    // rows read once stay through a refetch that fails: an event never wipes the wizard
    const failed =
      review.error?.message ??
      review.data?.error?.message ??
      (rows.data ? undefined : rows.error?.message) ??
      rows.data?.error?.message
    if (failed) return <p className="text-sm text-destructive">{sentence(failed)}</p>
    if (review.isPending || rows.isPending) return <Skeleton className="h-40 w-full" />

    if (nothing) return <NothingToPrune />

    if (step === 'worktrees')
      return <CandidateList candidates={candidates} isChecked={isChecked} onCheck={check} left={left} />

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
        {notice && <p className="text-sm text-amber-500">{notice}</p>}
        {unplanned && <p className="text-sm text-destructive">{sentence(unplanned)}</p>}
        <PlanWarnings warnings={warnings} />
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
                {count === 1 ? 'Prune' : `Prune ${count} worktrees`}
              </Button>
            )}
          </>
        )}
      </DialogFooter>
    </>
  )
}
