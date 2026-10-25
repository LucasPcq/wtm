import type { CreateRequest, DomainField, DomainForm } from '@/api/types.gen'

// The questions of create, as its request names them; remember and ask say how the others are answered, not what.
type Question = Exclude<CreateRequest[number]['path'], 'remember' | 'ask'>

const steps = ['branch', 'environment'] as const
type StepKey = (typeof steps)[number]

const stepTitle: Record<StepKey, string> = { branch: 'Branch', environment: 'Environment' }

// The step each question of create is asked in: a question create adds fails the build until it has one.
const stepOf: Record<Question, StepKey> = {
  branches: 'branch',
  from: 'branch',
  fast_forward: 'branch',
  env_from: 'environment',
  isolation: 'environment',
  if_not_exists: 'environment',
}

// A Step is one screen of the create dialog: its fields, those it asks and those the core does not ask, with why.
export type Step = { key: StepKey; title: string; fields: DomainField[]; asked: DomainField[]; skipped: DomainField[] }

const known = (path: string): path is Question => path in stepOf

// stepsOf is create's form as the dialog's steps, in their order; a field the build did not know of goes with the
// last, never dropped.
export function stepsOf(form: DomainForm): Step[] {
  return steps.map((key) => {
    const fields = form.fields.filter((field) => (known(field.path) ? stepOf[field.path] : steps.at(-1)) === key)
    return {
      key,
      title: stepTitle[key],
      fields,
      asked: fields.filter((field) => !field.skip),
      skipped: fields.filter((field) => field.skip),
    }
  })
}
