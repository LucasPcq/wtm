import type { DomainField, DomainForm } from '@/api/types.gen'
import type { Question } from '@/features/worktrees/create/utils/words'

const steps = ['branch', 'environment'] as const
export type StepKey = (typeof steps)[number]

const stepTitle: Record<StepKey, string> = { branch: 'Branch', environment: 'Environment' }

export const createSteps: Record<Question, StepKey> = {
  branches: 'branch',
  from: 'branch',
  fast_forward: 'branch',
  env_from: 'environment',
  isolation: 'environment',
  if_not_exists: 'environment',
}

type Field<P extends string> = DomainField & { path: P }

export type Step<P extends string = Question> = { title: string; asked: Field<P>[]; skipped: Field<P>[] }

// a field no step names is no question of the screen's: a flag the web never asks
export function stepsBy<P extends string>(form: DomainForm, of: Record<P, StepKey>): Step<P>[] {
  const questions = form.fields.filter((field): field is Field<P> => Object.hasOwn(of, field.path))
  return steps.map((key) => {
    const fields = questions.filter((field) => of[field.path] === key)
    return {
      title: stepTitle[key],
      asked: fields.filter((field) => !field.skip),
      skipped: fields.filter((field) => field.skip),
    }
  })
}

export const stepsOf = (form: DomainForm) => stepsBy(form, createSteps)

export function nextAsked(steps: Step<string>[], from: number): number {
  const ahead = steps.findIndex((step, i) => i > from && step.asked.length > 0)
  return ahead < 0 ? steps.length : ahead
}
