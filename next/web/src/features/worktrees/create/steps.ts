import type { DomainField, DomainForm } from '@/api/types.gen'

// A Step is one screen of the create dialog: a section of the form and its fields, those it asks and those the
// core does not ask, with why.
export type Step = { key: string; title: string; fields: DomainField[]; asked: DomainField[]; skipped: DomainField[] }

// stepsOf is the form as the dialog's steps: one per section of the form, in its order, a field of no section with
// the last; a form of no section is one step.
export function stepsOf(form: DomainForm): Step[] {
  const sections = form.sections.length > 0 ? form.sections : [{ key: '', title: 'Create' }]
  const last = sections.at(-1)?.key
  return sections.map((section) => {
    const fields = form.fields.filter((field) => form.sections.length === 0 || (field.section || last) === section.key)
    return {
      key: section.key,
      title: section.title,
      fields,
      asked: fields.filter((field) => !field.skip),
      skipped: fields.filter((field) => field.skip),
    }
  })
}
