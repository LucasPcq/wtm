import { RefreshCw } from 'lucide-react'
import { useReducer, useState } from 'react'
import type { DomainAnswered, DomainFollowUp, DomainWarning } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert } from '@/features/worktrees/components/alert'
import { CommandLine } from '@/features/worktrees/components/command-line'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import type { Left } from '@/features/worktrees/components/left-item'
import { LeftRow } from '@/features/worktrees/components/left-row'
import { LeftSection } from '@/features/worktrees/components/left-section'
import { PlanWarnings } from '@/features/worktrees/components/plan-warnings'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { Stepper } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { StackStep } from '@/features/worktrees/sync/components/stack-step'
import { SyncConfirm } from '@/features/worktrees/sync/components/sync-confirm'
import { useSyncPlan } from '@/features/worktrees/sync/hooks/use-sync-plan'
import { pushOnly, riskOf, unitsOf, valuesOf } from '@/features/worktrees/sync/utils/plan'
import { stackOf } from '@/features/worktrees/sync/utils/stack'
import { initialWizard, type Step, wizardReducer } from '@/features/worktrees/sync/utils/wizard-state'
import type { OutcomeRow } from '@/features/worktrees/utils/outcome'
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
  const [{ step, opening, checked, switches }, dispatch] = useReducer(
    wizardReducer,
    { answers: asked, step: opensOn },
    initialWizard,
  )
  const [query, setQuery] = useState('')
  const sync = useSyncPlan({ repo, asked, picking, checked, switches })
  const { review, reviewed, rows, plan, planned, refusal, fresh, empty, fingerprint, why } = sync
  const confirming = step === 'confirm'

  // any opener (a row, a follow-up) may name a worktree that needs consent: once the review lands, it opens step 1
  if (opening && review.isSuccess && !review.isFetching) dispatch({ type: 'opened', consent: sync.consent })

  const reparentable = (row: OutcomeRow): OutcomeRow => {
    const followUp = row.followUp
    if (followUp?.command !== 'reparent') return row
    const to = valuesOf(followUp.answers ?? [], 'to')?.[0] ?? 'its base'
    return { ...row, followUp: undefined, act: () => onReparent(followUp), actLabel: `Reparent onto ${to}` }
  }
  const cantSync = sync.cantSync.map(reparentable)

  const primary = () => {
    if (plan && pushOnly(plan)) return sync.count > 1 ? `Push ${sync.count} worktrees` : 'Push'
    return sync.count > 1 ? `Sync ${sync.count} worktrees` : 'Sync'
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
    if (review.isPending || (confirming && sync.asking && !planned.data)) return <Skeleton className="h-40 w-full" />
    if (review.isError) return <Alert tone="red" description={review.error.message} />
    if (planned.isError) return <Alert tone="red" description={planned.error.message} />
    if (review.data.error) return refused()

    if (!confirming)
      return (
        <>
          {refused()}
          <LeftSection label="Can't sync">
            {cantSync.map((row) => (
              <LeftRow key={row.key} row={row} />
            ))}
          </LeftSection>
          <StackStep
            stack={stackOf(reviewed ?? { onto: [], worktrees: [] })}
            plan={plan}
            picking={picking}
            isNamed={sync.isNamed}
            onCheck={(list, on) =>
              dispatch({ type: 'check', rows: list.map((r) => ({ worktree: r.worktree, risk: riskOf(r) })), on })
            }
            ontoTicked={sync.ontoTicked}
            onOntoTick={(onto, on) =>
              dispatch({ type: 'check', rows: [{ worktree: onto.branch, risk: sync.ontoRisk(onto) }], on })
            }
            changesOf={sync.changesOf}
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
    // nothing to plan: say what is missing, never a skeleton that waits for ever
    if (!plan) return sync.asking ? <Skeleton className="h-40 w-full" /> : <Alert tone="amber" title={why()} />
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
        reviewWarnings={review.data?.warnings ?? []}
        left={[
          ...cantSync,
          ...left.map(({ worktree, why: reason }) => ({
            key: worktree,
            tone: 'muted' as const,
            title: worktree,
            message: reason,
          })),
        ]}
        review={rows}
        behind={sync.behind}
        changesOf={sync.changesOf}
        onReparent={onReparent}
      />
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Sync worktrees</DialogTitle>
        <DialogDescription>
          {plan && pushOnly(plan)
            ? 'Each worktree is pushed to origin, with force-with-lease.'
            : 'Each worktree is rebased onto its parent, parents first.'}
        </DialogDescription>
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
                onClick={() =>
                  plan && fingerprint && onStart({ answers: sync.answers, fingerprint, units: unitsOf(plan) })
                }
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
