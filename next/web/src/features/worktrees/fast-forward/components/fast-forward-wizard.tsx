import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { FastForward } from 'lucide-react'
import { useReducer, useState } from 'react'
import { dryRunQuery, planQuery, questionsQuery } from '@/api/queries/commands'
import type { DomainOption, DomainWarning, FastforwardAdvance } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { Details } from '@/features/worktrees/components/details'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import type { Left } from '@/features/worktrees/components/left-item'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { Stepper } from '@/features/worktrees/components/stepper'
import { AdvanceItem } from '@/features/worktrees/fast-forward/components/advance-item'
import { LeftAdvance } from '@/features/worktrees/fast-forward/components/left-advance'
import { PickerStep } from '@/features/worktrees/fast-forward/components/picker-step'
import {
  behind,
  moves,
  noFetch,
  requestOf,
  riskOf,
  unitsOf,
  unsafe,
} from '@/features/worktrees/fast-forward/utils/plan'
import { leftRow } from '@/features/worktrees/fast-forward/utils/recap'
import { initialWizard, wizardReducer } from '@/features/worktrees/fast-forward/utils/wizard-state'
import { commits } from '@/features/worktrees/fast-forward/utils/words'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { planWide } from '@/features/worktrees/utils/outcome'

export function FastForwardWizard({
  repo,
  worktrees,
  left,
  picking,
  notice,
  onStart,
  onCancel,
}: {
  repo: string
  worktrees: string[]
  left: Left[]
  picking: boolean
  notice?: DomainWarning | undefined
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const [{ step, checked, named, chosen }, dispatch] = useReducer(wizardReducer, { picking, worktrees }, initialWizard)
  const [query, setQuery] = useState('')
  const confirming = step === 'confirm'

  // a tick holds for the risk it was given under, an untick whatever the risk: a worktree the person left out never
  // comes back on its own
  const ticked = (worktree: string, risk: string) => {
    const tick = checked[worktree]
    return tick && (!tick.on || tick.risk === risk) ? tick : undefined
  }

  // the questions never fetch: the picker shows where each stands as origin was last fetched
  const form = useQuery({ ...questionsQuery(repo, 'fast-forward'), enabled: picking })
  const field = form.data?.fields.find((one) => one.path === 'worktree')
  const options = field?.kind === 'multi_select' ? field.options : []
  const prechecked = field?.kind === 'multi_select' ? field.picked : []
  const optionRisk = (option: DomainOption) => (option.changes ? 'dirty' : '')
  const isPicked = (option: DomainOption) =>
    ticked(option.value, optionRisk(option))?.on ?? prechecked.includes(option.value)
  const picks = options.filter(isPicked).map((option) => option.value)
  const untouched = picks.length === prechecked.length && picks.every((name) => prechecked.includes(name))

  // named, the review is forced: it plans every worktree with what keeps it, and the choices are the person's;
  // chosen, it is the command's own plan, the one applied
  const review = useQuery({
    ...dryRunQuery(
      repo,
      'fast-forward',
      chosen
        ? []
        : [
            { path: 'worktree', values: named },
            { path: 'force', values: ['true'] },
          ],
    ),
    enabled: confirming,
  })
  const advances = review.data?.plan?.worktrees ?? []
  const listed = advances.filter((advance) => behind(advance) && (!chosen || moves(advance)))
  const leftAdvances = advances.filter((advance) => !listed.includes(advance))
  const isChecked = (advance: FastforwardAdvance) =>
    chosen || (ticked(advance.worktree, riskOf(advance))?.on ?? !unsafe(advance))
  const going = listed.filter(isChecked)
  // the review fetched origin: every plan made again on a choice reads it as the review left it
  const answers = [...requestOf({ chosen, going, left: leftAdvances }), noFetch]
  const planned = useQuery({
    ...planQuery(repo, 'fast-forward', { answers, readAt: review.dataUpdatedAt }),
    enabled: confirming && going.length > 0 && !review.isFetching,
    placeholderData: keepPreviousData,
  })
  const plan = planned.isError || planned.data?.error ? undefined : planned.data?.plan
  const fingerprint = planned.data?.fingerprint
  // never forced past what the person checked under that very risk: the two reads land in any order
  const consented = (plan?.worktrees ?? []).every(
    (advance) => !unsafe(advance) || !moves(advance) || ticked(advance.worktree, riskOf(advance))?.on,
  )
  const fresh =
    plan !== undefined && !planned.isPlaceholderData && !planned.isFetching && !review.isFetching && consented
  const unplanned = planned.isError ? planned.error.message : planned.data?.error?.message

  const warningOf = (worktree: string) => review.data?.warnings.find((warning) => warning.params?.worktree === worktree)
  const leftRows = [
    ...leftAdvances.map((advance) => (
      <LeftAdvance key={advance.worktree} row={leftRow(advance, warningOf(advance.worktree))} />
    )),
    ...left.map(({ worktree, why }) => (
      <LeftAdvance key={worktree} row={{ key: worktree, tone: 'muted', title: worktree, message: why }} />
    )),
  ]
  const empty = review.isSuccess && !review.data.error && review.data.plan?.empty === true
  // kept for its changes, a worktree behind origin has something to take all the same
  const kept = leftAdvances.some(unsafe)

  const why = () => {
    if (review.isFetching) return 'Fetching origin'
    // a plan made again on a tick reads origin as the review fetched it
    if (planned.isFetching) return 'Planning'
    // the plan saw a worktree change before the review did: the event that refreshes it is on its way
    if (!consented) return 'Checking the worktrees again'
    if (going.length > 0) return undefined
    const held = listed.filter(unsafe).map((advance) => advance.worktree)
    if (held.length > 0 && held.length === listed.length)
      return `Check ${held.join(', ')} to fast-forward ${held.length === 1 ? 'it' : 'them'} anyway`
    return 'Check a worktree to fast-forward it'
  }

  const body = () => {
    if (!confirming) {
      if (form.isPending) return <Skeleton className="h-40 w-full" />
      if (form.isError) return <Alert tone="red" description={form.error.message} />
      return (
        <PickerStep
          options={options}
          isChecked={isPicked}
          onCheck={(rows, on) =>
            dispatch({ type: 'check', rows: rows.map((o) => ({ worktree: o.value, risk: optionRisk(o) })), on })
          }
          query={query}
          onQuery={setQuery}
        />
      )
    }

    if (review.isPending) return <Skeleton className="h-32 w-full" />
    if (review.isError) return <Alert tone="red" description={review.error.message} />
    if (review.data.error) return <Alert tone="red" description={review.data.error.message} />

    if (empty)
      return (
        <>
          <PlanWarnings warnings={review.data.warnings.filter(planWide)} />
          <Empty className="gap-4 p-6">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <FastForward />
              </EmptyMedia>
              <EmptyTitle>Nothing to fast-forward</EmptyTitle>
              <EmptyDescription>
                {kept
                  ? 'What origin has waits on uncommitted changes, kept as they are.'
                  : 'Origin has nothing these worktrees lack.'}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
          <ItemList left={leftRows}>{[]}</ItemList>
        </>
      )

    return (
      <>
        <PlanWarnings warnings={(planned.data ?? review.data).warnings.filter(planWide)} />
        <ItemList label="Worktrees" left={leftRows}>
          {listed.map((advance) => (
            <AdvanceItem
              key={advance.worktree}
              advance={advance}
              checked={isChecked(advance)}
              {...(!chosen && {
                onCheck: (on: boolean) =>
                  dispatch({ type: 'check', rows: [{ worktree: advance.worktree, risk: riskOf(advance) }], on }),
              })}
            />
          ))}
        </ItemList>
        {going.length > 0 && (
          <Details label="Details" summary="where each moves">
            {going.map((advance) => (
              <Facts
                key={advance.worktree}
                heading={advance.worktree}
                facts={[
                  {
                    label: 'Moves',
                    values: [
                      `${advance.from.slice(0, 7)} → ${advance.to?.slice(0, 7)} · ${commits(advance.origin?.behind ?? 0)}`,
                    ],
                    mono: true,
                  },
                  { label: 'Folder', values: [advance.path], mono: true },
                ]}
              />
            ))}
          </Details>
        )}
      </>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Fast-forward worktrees</DialogTitle>
        <DialogDescription>Each moves to origin's tip; nothing is merged or rebased.</DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper
          steps={[
            { title: 'Worktrees', state: confirming ? 'done' : 'now' },
            { title: 'Confirm', state: confirming ? 'now' : 'todo' },
          ]}
        />
      </DialogBand>

      <DialogScroll>
        {notice && <PlanWarnings warnings={[notice]} />}
        {confirming && unplanned && <Alert tone="red" description={unplanned} />}
        {body()}
      </DialogScroll>

      <DialogFooter className="items-center">
        {empty ? (
          <Button onClick={onCancel}>Done</Button>
        ) : (
          <>
            <Button variant="ghost" className="mr-auto" onClick={onCancel}>
              Cancel
            </Button>
            {picking && confirming && (
              <Button variant="outline" onClick={() => dispatch({ type: 'back' })}>
                Back
              </Button>
            )}
            {!confirming && (
              <ReasonButton
                disabled={picks.length === 0}
                why={form.data ? 'Check a worktree to fast-forward it' : undefined}
                onClick={() => dispatch({ type: 'confirm', named: picks, chosen: untouched })}
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
                {going.length > 1 ? `Fast-forward ${going.length} worktrees` : 'Fast-forward'}
              </ReasonButton>
            )}
          </>
        )}
      </DialogFooter>
    </>
  )
}
