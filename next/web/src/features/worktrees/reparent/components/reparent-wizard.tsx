import { useQuery } from '@tanstack/react-query'
import { useReducer } from 'react'
import { dryRunQuery, questionsQuery } from '@/api/queries/commands'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Item, ItemContent, ItemDescription, ItemTitle } from '@/components/ui/item'
import { Skeleton } from '@/components/ui/skeleton'
import { Details } from '@/features/worktrees/components/details'
import { DialogBand } from '@/features/worktrees/components/dialog-band'
import { DialogScroll } from '@/features/worktrees/components/dialog-scroll'
import { Facts } from '@/features/worktrees/components/facts'
import { ItemList } from '@/features/worktrees/components/item-list'
import { type Left, LeftItem } from '@/features/worktrees/components/left-item'
import { ReasonButton } from '@/features/worktrees/components/reason-button'
import { RunFailure } from '@/features/worktrees/components/run-failure'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import type { Start } from '@/features/worktrees/hooks/use-run'
import { ParentStep } from '@/features/worktrees/reparent/components/parent-step'
import { WorktreesStep } from '@/features/worktrees/reparent/components/worktrees-step'
import { unchanged, unitsOf } from '@/features/worktrees/reparent/utils/plan'
import { initialWizard, type Step, wizardReducer } from '@/features/worktrees/reparent/utils/wizard-state'
import { followWords, moveWords, parentWords, refusalOf, title } from '@/features/worktrees/reparent/utils/words'
import { requestOf } from '@/features/worktrees/utils/reparent'

const steps: { step: Step; title: string }[] = [
  { step: 'worktrees', title: 'Worktrees' },
  { step: 'parent', title: 'New parent' },
  { step: 'confirm', title: 'Confirm' },
]

export function ReparentWizard({
  repo,
  worktrees,
  left,
  picking,
  to: given,
  onStart,
  onCancel,
}: {
  repo: string
  worktrees: string[]
  left: Left[]
  picking: boolean
  to: string | undefined
  onStart: (start: Start) => void
  onCancel: () => void
}) {
  const [{ step, checked, to }, dispatch] = useReducer(wizardReducer, { picking, worktrees, to: given }, initialWizard)
  const at = steps.findIndex((one) => one.step === step)

  // each worktree reparent can move, with the parent it stands on
  const movable = useQuery(questionsQuery(repo, 'reparent'))
  const worktreeField = movable.data?.fields.find((field) => field.path === 'worktree')
  const options = (worktreeField?.kind === 'multi_select' ? worktreeField.options : []).filter((option) =>
    worktrees.includes(option.value),
  )
  const moving = picking ? checked.filter((worktree) => options.some((option) => option.value === worktree)) : checked

  const parents = useQuery({
    ...questionsQuery(repo, 'reparent', [{ path: 'worktree', values: moving }]),
    enabled: step === 'parent' && moving.length > 0,
  })
  const toField = parents.data?.fields.find((field) => field.path === 'to')

  const answers = requestOf({ worktrees: moving, to })
  const review = useQuery({ ...dryRunQuery(repo, 'reparent', answers), enabled: step === 'confirm' })
  const plan = review.data?.error ? undefined : review.data?.plan
  const moves = plan?.moves ?? []
  const going = moves.filter((move) => !unchanged(move))
  const fingerprint = review.data?.fingerprint

  const parentLine = () => {
    const [only] = moving
    const option = options.find((one) => one.value === only)
    if (moving.length !== 1 || !option) return undefined
    if (!option.parent) return `${only} has no parent.`
    return `${only} is on ${parentWords({ from: option.parent, parent_gone: option.gone ?? false })}.`
  }

  const why = () => {
    if (review.isFetching) return 'Planning'
    if (review.data?.error) return refusalOf(review.data.error)
    if (!plan?.empty) return undefined
    const [only] = moves
    if (moves.length === 1 && only) return `${only.branch} is already on ${only.to}`
    return `Each is already on ${to}`
  }

  const body = () => {
    if (step === 'worktrees') {
      if (movable.isPending) return <Skeleton className="h-40 w-full" />
      if (movable.isError) return <RunFailure message={movable.error.message} />
      return (
        <WorktreesStep
          options={options}
          checked={moving}
          left={left}
          onCheck={(names, on) => dispatch({ type: 'check', worktrees: names, on })}
        />
      )
    }

    if (step === 'parent') {
      if (parents.isPending) return <Skeleton className="h-20 w-full" />
      if (parents.isError) return <RunFailure message={parents.error.message} />
      if (toField?.kind !== 'select') return null
      return (
        <ParentStep
          field={toField}
          description={parentLine()}
          value={to}
          onChange={(next) => dispatch({ type: 'to', to: next })}
        />
      )
    }

    if (review.isPending) return <Skeleton className="h-32 w-full" />
    if (review.isError) return <RunFailure message={review.error.message} />
    if (review.data.error) return <RunFailure message={refusalOf(review.data.error)} />
    return (
      <>
        <ItemList
          label="Worktrees"
          left={moves
            .filter(unchanged)
            .map((move) => <LeftItem key={move.branch} worktree={move.branch} why={`already on ${move.to}`} />)}
        >
          {going.map((move) => (
            <Item key={move.branch} role="listitem" aria-label={move.branch} className="rounded-none px-4 py-3">
              <ItemContent className="gap-1">
                <ItemTitle>{move.branch}</ItemTitle>
                <ItemDescription className="text-xs">{moveWords(move)}</ItemDescription>
                {followWords(move.children ?? []) && (
                  <ItemDescription className="text-xs">{followWords(move.children ?? [])}</ItemDescription>
                )}
              </ItemContent>
            </Item>
          ))}
        </ItemList>
        {going.length > 0 && (
          <Details label="Details" summary={going.length === 1 ? 'its folder' : 'their folders'}>
            {going.map((move) => (
              <Facts
                key={move.branch}
                heading={move.branch}
                facts={[{ label: 'Folder', values: [move.path], mono: true }]}
              />
            ))}
          </Details>
        )}
      </>
    )
  }

  const stateOf = (index: number): StepState => {
    if (index === 0 && !picking) return 'done'
    if (index < at) return 'done'
    return index === at ? 'now' : 'todo'
  }

  const next = () => {
    if (step === 'worktrees')
      return (
        <ReasonButton
          disabled={moving.length === 0}
          why="Check a worktree to reparent it"
          onClick={() => dispatch({ type: 'next' })}
        >
          Next
        </ReasonButton>
      )
    if (step === 'parent')
      return (
        <ReasonButton disabled={to === ''} why="Choose a new parent" onClick={() => dispatch({ type: 'next' })}>
          Next
        </ReasonButton>
      )
    return (
      <ReasonButton
        disabled={why() !== undefined || !plan || !fingerprint}
        why={why()}
        onClick={() => plan && fingerprint && onStart({ answers, fingerprint, units: unitsOf(plan) })}
      >
        {going.length > 1 ? `Reparent ${going.length} worktrees` : 'Reparent'}
      </ReasonButton>
    )
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription>
          {moving.length > 1
            ? 'Each rebases onto the new parent on its next sync.'
            : 'It rebases onto the new parent on its next sync.'}
        </DialogDescription>
      </DialogHeader>

      <DialogBand>
        <Stepper steps={steps.map((one, i) => ({ title: one.title, state: stateOf(i) }))} />
      </DialogBand>

      <DialogScroll>{body()}</DialogScroll>

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {(step === 'confirm' || (step === 'parent' && picking)) && (
          <Button variant="outline" onClick={() => dispatch({ type: 'back' })}>
            Back
          </Button>
        )}
        {next()}
      </DialogFooter>
    </>
  )
}
