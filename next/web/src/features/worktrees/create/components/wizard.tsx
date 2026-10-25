import { useReducer } from 'react'
import type { DomainAnswered, DomainForm } from '@/api/types.gen'
import { keyOf, problemsByKey } from '@/components/command-form/answers'
import { CommandFields } from '@/components/command-form/command-fields'
import { useCommandForm } from '@/components/command-form/use-command-form'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Stepper, type StepState } from '@/features/worktrees/components/stepper'
import { ApplyButton } from '@/features/worktrees/create/components/apply-button'
import { Confirm } from '@/features/worktrees/create/components/confirm'
import { NotAsked } from '@/features/worktrees/create/components/not-asked'
import type { Planned } from '@/features/worktrees/create/utils/progress'
import { nextAsked, stepsOf } from '@/features/worktrees/create/utils/steps'
import { initialWizard, wizardReducer } from '@/features/worktrees/create/utils/wizard-state'
import { createWords } from '@/features/worktrees/create/utils/words'

export function Wizard({
  repo,
  form: initial,
  onStart,
  onCancel,
}: {
  repo: string
  form: DomainForm
  onStart: (start: Planned) => void
  onCancel: () => void
}) {
  const { form, asked, answers, check, problems } = useCommandForm({ repo, command: 'create', form: initial })
  const [{ at, remember }, dispatch] = useReducer(wizardReducer, initial, initialWizard)

  const steps = stepsOf(asked)
  const step = steps[at]
  const confirming = at >= steps.length
  const request: DomainAnswered[] = [
    ...answers(),
    ...(remember.length > 0 ? [{ path: 'remember', values: remember }] : []),
  ]
  const previous = Math.max(
    steps.findLastIndex((candidate, i) => i < at && candidate.asked.length > 0),
    0,
  )

  const state = (i: number): StepState => {
    if (i === at) return 'now'
    if (i > at) return 'todo'
    return steps[i]?.asked.length === 0 ? 'skipped' : 'done'
  }

  // Next asks the core with the answers as typed, not as last checked, and goes by the steps of that answer: a step
  // with problems shows them rather than a Next that cannot be pressed
  const next = async () => {
    const answered = await check()
    const fresh = stepsOf(answered)
    const filed = problemsByKey(answered.fields, answered.problems)
    const here = fresh[at]
    if (!here?.asked.some((field) => filed[keyOf(field.path)]))
      return dispatch({ type: 'go', to: nextAsked(fresh, at) })

    for (const field of here.asked) form.setFieldMeta(keyOf(field.path), (meta) => ({ ...meta, isTouched: true }))
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Create worktree</DialogTitle>
        <DialogDescription>Each branch gets its own folder, .env and ports.</DialogDescription>
      </DialogHeader>

      <div className="border-b pb-4">
        <Stepper
          steps={[
            ...steps.map(({ title }, i) => ({ title, state: state(i) })),
            { title: 'Confirm', state: confirming ? 'now' : 'todo' },
          ]}
        />
      </div>

      {step && (
        <div className="flex flex-col gap-6">
          <CommandFields
            form={form}
            fields={step.asked}
            words={createWords}
            problems={problems}
            remember={{ paths: remember, toggle: (path) => dispatch({ type: 'remember', path }) }}
          />
          {problems[''] && <p className="text-sm text-destructive">{problems['']}</p>}
          <NotAsked fields={step.skipped} />
        </div>
      )}

      {confirming && <Confirm repo={repo} answers={request} />}

      <DialogFooter className="items-center">
        <Button variant="ghost" className="mr-auto" onClick={onCancel}>
          Cancel
        </Button>
        {at > 0 && (
          <Button variant="outline" onClick={() => dispatch({ type: 'go', to: previous })}>
            Back
          </Button>
        )}
        {!confirming && (
          <form.Subscribe selector={(s) => s.isValidating}>
            {(validating) => (
              <Button disabled={validating} onClick={next}>
                Next
              </Button>
            )}
          </form.Subscribe>
        )}
        {confirming && <ApplyButton repo={repo} answers={request} onStart={onStart} />}
      </DialogFooter>
    </>
  )
}
