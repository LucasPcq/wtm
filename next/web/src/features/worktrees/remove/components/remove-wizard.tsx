import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { dryRunQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { ChildrenStep } from '@/features/worktrees/remove/components/children-step'
import { WorktreeCard } from '@/features/worktrees/remove/components/worktree-card'
import { type Choice, requestOf, unitsOf, unsafe } from '@/features/worktrees/remove/utils/plan'

type Step = 'worktrees' | 'children' | 'confirm'

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
  const [step, setStep] = useState<Step>('worktrees')
  const [choices, setChoices] = useState<Record<string, Choice>>({})
  const [reparent, setReparent] = useState(false)

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
  const plan = planned.data?.error ? undefined : planned.data?.plan
  const moves = plan?.moves ?? []
  const fingerprint = planned.data?.fingerprint
  // the buttons never act on the plan a choice is replacing
  const fresh = plan !== undefined && !planned.isPlaceholderData
  const safe = removals.filter((removal) => !unsafe(removal))

  const state = (of: Step): StepState => {
    const order: Step[] = ['worktrees', 'children', 'confirm']
    if (of === step) return 'now'
    if (of === 'children' && moves.length === 0 && step === 'confirm') return 'skipped'
    return order.indexOf(of) < order.indexOf(step) ? 'done' : 'todo'
  }

  const next = () => {
    if (step === 'worktrees') return setStep(moves.length > 0 ? 'children' : 'confirm')
    setStep('confirm')
  }

  const back = () => setStep(step === 'confirm' && moves.length > 0 ? 'children' : 'worktrees')

  const status = () => {
    if (undecided.length > 0)
      return `${undecided.length} of ${removals.length} can't be removed safely: choose for each before going on.`
    if (going.length === 0) return 'Every worktree is kept: nothing to remove.'
    return undefined
  }

  const body = () => {
    if (review.isPending) return <Skeleton className="h-40 w-full" />
    if (review.isError) return <p className="text-sm text-destructive">{review.error.message}</p>
    if (review.data.error) return <p className="text-sm text-destructive">{review.data.error.message}</p>

    if (step === 'worktrees')
      return (
        <div className="flex flex-col gap-3">
          <div className="flex items-baseline gap-2">
            <h3 className="flex-1 font-medium">
              {removals.length === 1 ? `Remove ${removals[0]?.worktree}` : `Remove ${removals.length} worktrees`}
            </h3>
            {removals.length > 1 && removals.length !== safe.length && (
              <span className="text-[13px] text-muted-foreground">
                {safe.length} safe · {removals.length - safe.length} unsafe
              </span>
            )}
          </div>
          {status() && <p className="rounded-md bg-muted px-3 py-2.5 text-[13px]">{status()}</p>}
          {[...removals.filter(unsafe), ...safe].map((removal) => (
            <WorktreeCard
              key={removal.worktree}
              removal={removal}
              choice={choices[removal.worktree]}
              onChoice={(choice) => setChoices({ ...choices, [removal.worktree]: choice })}
            />
          ))}
        </div>
      )

    if (step === 'children') return <ChildrenStep moves={moves} reparent={reparent} onReparent={setReparent} />

    if (!plan) return <Skeleton className="h-40 w-full" />
    return (
      <div className="flex flex-col gap-3">
        <h3 className="font-medium">
          {going.length === 1 ? `Remove ${going[0]}` : `Remove ${going.slice(0, -1).join(', ')} and ${going.at(-1)}`}
        </h3>
        {(plan.worktrees ?? []).map((removal) => (
          <WorktreeCard
            key={removal.worktree}
            removal={removal}
            moves={moves.filter((move) => move.from === removal.branch)}
            reparent={reparent}
          />
        ))}
      </div>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Remove worktrees</DialogTitle>
        <DialogDescription>Each goes with its local branch and its record; the remote branch stays.</DialogDescription>
      </DialogHeader>

      <div className="border-b pb-4">
        <Stepper
          steps={[
            { title: 'Worktrees', state: state('worktrees') },
            { title: 'Children', state: state('children') },
            { title: 'Confirm', state: state('confirm') },
          ]}
        />
      </div>

      <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">{body()}</div>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {step !== 'worktrees' && (
          <Button variant="outline" onClick={back}>
            Back
          </Button>
        )}
        {step !== 'confirm' && (
          <Button disabled={!fresh} onClick={next}>
            Next
          </Button>
        )}
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
