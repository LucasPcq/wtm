import { useReducer } from 'react'
import type { ApiCommandName, DomainAnswered, DomainForm } from '@/api/types.gen'
import { keyOf, problemsByKey } from '@/components/command-form/answers'
import { useCommandForm } from '@/components/command-form/use-command-form'
import type { StepState } from '@/features/worktrees/components/stepper'
import { nextAsked, type StepKey, stepsBy } from '@/features/worktrees/create/utils/steps'
import { initialWizard, wizardReducer } from '@/features/worktrees/create/utils/wizard-state'

// a wizard over a command's questions, its steps by the paths stepOf places, then Confirm
export function useWizard<P extends string>({
  repo,
  command,
  form: initial,
  stepOf,
  extend,
  onAnswered,
}: {
  repo: string
  command: ApiCommandName
  form: DomainForm
  stepOf: Record<P, StepKey>
  extend?: (answers: DomainAnswered[]) => DomainAnswered[]
  onAnswered?: (sent: DomainAnswered[]) => void
}) {
  const commandForm = useCommandForm({
    repo,
    command,
    form: initial,
    ...(extend && { extend }),
    ...(onAnswered && { onAnswered }),
  })
  const { form, asked, answers, check } = commandForm
  const [{ at, remember }, dispatch] = useReducer(wizardReducer, initial, initialWizard)

  const steps = stepsBy(asked, stepOf)
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
    const fresh = stepsBy(answered, stepOf)
    const filed = problemsByKey(answered.fields, answered.problems)
    const here = fresh[at]
    if (!here?.asked.some((field) => filed[keyOf(field.path)]))
      return dispatch({ type: 'go', to: nextAsked(fresh, at) })

    for (const field of here.asked) form.setFieldMeta(keyOf(field.path), (meta) => ({ ...meta, isTouched: true }))
  }

  return {
    ...commandForm,
    at,
    step: steps[at],
    confirming,
    request,
    stepper: [
      ...steps.map(({ title }, i) => ({ title, state: state(i) })),
      { title: 'Confirm', state: confirming ? ('now' as const) : ('todo' as const) },
    ],
    remember: { paths: remember, toggle: (path: string) => dispatch({ type: 'remember', path }) },
    back: () => dispatch({ type: 'go', to: previous }),
    next,
  }
}
