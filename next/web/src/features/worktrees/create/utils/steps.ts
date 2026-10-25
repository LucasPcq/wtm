import type { DomainForm } from '@/api/types.gen'
import { type Asked, isAsked, type Question } from '@/features/worktrees/create/utils/words'

const steps = ['branch', 'environment'] as const
type StepKey = (typeof steps)[number]

const stepTitle: Record<StepKey, string> = { branch: 'Branch', environment: 'Environment' }

const stepOf: Record<Question, StepKey> = {
  branches: 'branch',
  from: 'branch',
  fast_forward: 'branch',
  env_from: 'environment',
  isolation: 'environment',
  if_not_exists: 'environment',
}

export type Step = { title: string; asked: Asked[]; skipped: Asked[] }

export function stepsOf(form: DomainForm): Step[] {
  const questions = form.fields.filter(isAsked)
  return steps.map((key) => {
    const fields = questions.filter((field) => stepOf[field.path] === key)
    return {
      title: stepTitle[key],
      asked: fields.filter((field) => !field.skip),
      skipped: fields.filter((field) => field.skip),
    }
  })
}

export function nextAsked(steps: Step[], from: number): number {
  const ahead = steps.findIndex((step, i) => i > from && step.asked.length > 0)
  return ahead < 0 ? steps.length : ahead
}
