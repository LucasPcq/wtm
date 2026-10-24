import type { DomainAnswered, DomainField, DomainFieldError } from '../api/types.gen'

// A form's values are keyed by the index of their field: a path such as remember[0] would read as nested to the
// form library. Each value is the answer as the catalogue takes it, an array of strings.
export type Values = Record<string, string[]>

export const keyOf = (index: number) => `f${index}`

// valuesOf is what each field holds as the form stands, the core's defaults included.
export function valuesOf(fields: DomainField[]): Values {
  return Object.fromEntries(fields.map((field, index) => [keyOf(index), valueOf(field)]))
}

function valueOf(field: DomainField): string[] {
  switch (field.kind) {
    case 'select':
    case 'text':
      return field.value ? [field.value] : []
    case 'multi_select':
      return field.picked
    case 'text_list':
      return field.entries
    case 'confirm':
      return [field.value ? field.yes.value : field.no.value]
  }
}

// answersOf is the request a form's values make: only the fields the person changed, so that what they left to
// its default (a remembered answer included) stays the core's to decide.
export function answersOf(fields: DomainField[], values: Values, changed: (key: string) => boolean): DomainAnswered[] {
  return fields.flatMap((field, index) => {
    const key = keyOf(index)
    return changed(key) ? [{ path: field.path, values: values[key] ?? [] }] : []
  })
}

// problemsByKey files each problem of the core under the key of its field, and a problem no field holds under ''.
export function problemsByKey(fields: DomainField[], problems: DomainFieldError[]): Record<string, string> {
  const byPath = new Map(fields.map((field, index) => [field.path, keyOf(index)]))
  const filed: Record<string, string> = {}
  for (const problem of problems) {
    const key = byPath.get(problem.path) ?? ''
    filed[key] = filed[key] ? `${filed[key]}\n${problem.message}` : problem.message
  }
  return filed
}

// problemsOf asks the core the questions again with the answers the person changed, and returns its problems
// filed for the form: under each field's key, and under form those of no field. The front checks nothing itself.
export async function problemsOf(params: {
  fields: DomainField[]
  values: Values
  changed: (key: string) => boolean
  ask: (answers: DomainAnswered[]) => Promise<{ problems: DomainFieldError[] }>
}): Promise<{ form?: string; fields: Record<string, string> } | undefined> {
  const asked = await params.ask(answersOf(params.fields, params.values, params.changed))
  const { '': form, ...fields } = problemsByKey(params.fields, asked.problems)
  return form || Object.keys(fields).length ? { form, fields } : undefined
}
